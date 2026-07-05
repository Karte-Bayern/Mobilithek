package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/karte-bayern/mobilithek"
)

type headerFlags []string

func (h *headerFlags) String() string {
	return strings.Join(*h, ",")
}

func (h *headerFlags) Set(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New("header must not be empty")
	}
	*h = append(*h, value)
	return nil
}

func main() {
	log.SetFlags(0)

	subscriptionID := flag.String("subscription-id", os.Getenv("MOBILITHEK_SUBSCRIPTION_ID"), "Mobilithek subscription ID")
	endpoint := flag.String("endpoint", "auto", "endpoint: auto, generic, datex2-v2, datex2-v3, container")
	out := flag.String("out", "-", "output file path, or - for stdout")
	geojsonOut := flag.String("geojson-out", "", "optional GeoJSON output path converted from the XML response")
	certFile := flag.String("cert", os.Getenv("MOBILITHEK_CERT_FILE"), "client certificate file")
	keyFile := flag.String("key", os.Getenv("MOBILITHEK_KEY_FILE"), "client key file")
	caFile := flag.String("ca", os.Getenv("MOBILITHEK_CA_FILE"), "optional root CA PEM file")
	baseURL := flag.String("base-url", mobilithek.DefaultBaseURL, "Mobilithek base URL")
	timeout := flag.Duration("timeout", 60*time.Second, "request timeout")
	maxBytes := flag.Int64("max-bytes", 250*1024*1024, "maximum response size")
	etag := flag.String("etag", os.Getenv("MOBILITHEK_ETAG"), "send If-None-Match with this ETag")
	ifModifiedSince := flag.String("if-modified-since", os.Getenv("MOBILITHEK_IF_MODIFIED_SINCE"), "send If-Modified-Since as an HTTP-date")
	var extraHeaders headerFlags
	flag.Var(&extraHeaders, "header", "additional request header as 'Name: value' or 'Name=value'; repeatable")
	flag.Parse()

	if *subscriptionID == "" {
		log.Fatal("missing -subscription-id or MOBILITHEK_SUBSCRIPTION_ID")
	}

	client, err := mobilithek.New(
		mobilithek.WithBaseURL(*baseURL),
		mobilithek.WithClientCertificate(*certFile, *keyFile),
		mobilithek.WithRootCA(*caFile),
		mobilithek.WithTimeout(*timeout),
		mobilithek.WithMaxBytes(*maxBytes),
	)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	headers, err := requestHeaders(*etag, *ifModifiedSince, extraHeaders)
	if err != nil {
		log.Fatal(err)
	}

	// Conditional requests let repeated pulls avoid re-downloading unchanged
	// subscription data when Mobilithek returns HTTP 304.
	response, err := client.FetchSubscriptionWithHeaders(ctx, *subscriptionID, mobilithek.EndpointKind(*endpoint), headers)
	if err != nil {
		log.Fatal(err)
	}
	if response.StatusCode == http.StatusNotModified {
		fmt.Fprintf(os.Stderr, "not modified (HTTP 304, url=%s, etag=%s, last-modified=%s)\n", response.URL, response.ETag(), response.LastModified())
		return
	}
	if !response.OK() {
		log.Fatalf("HTTP %d from %s", response.StatusCode, response.URL)
	}

	if *out == "-" {
		_, _ = os.Stdout.Write(response.Body)
		if *geojsonOut != "" {
			if err := writeGeoJSON(*geojsonOut, response.Body, *subscriptionID); err != nil {
				log.Fatal(err)
			}
		}
		return
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, response.Body, 0o644); err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes, content-type=%s, url=%s)\n", *out, len(response.Body), response.ContentType(), response.URL)

	if *geojsonOut != "" {
		if err := writeGeoJSON(*geojsonOut, response.Body, *subscriptionID); err != nil {
			log.Fatal(err)
		}
	}
}

func writeGeoJSON(path string, body []byte, subscriptionID string) error {
	geojson, err := mobilithek.GeoJSONFromDATEX2XML(body, "subscription-"+subscriptionID)
	if err != nil {
		return err
	}

	output, err := mobilithek.MarshalGeoJSON(geojson, true)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	if err := os.WriteFile(path, output, 0o644); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%d features)\n", path, len(geojson.Features))
	return nil
}

func requestHeaders(etag string, ifModifiedSince string, extraHeaders []string) (map[string]string, error) {
	headers := map[string]string{}
	if etag = strings.TrimSpace(etag); etag != "" {
		headers["If-None-Match"] = etag
	}
	if ifModifiedSince = strings.TrimSpace(ifModifiedSince); ifModifiedSince != "" {
		// HTTP validators must use the RFC 7231 HTTP-date format; accepting
		// arbitrary date strings would make cache behavior ambiguous.
		if _, err := http.ParseTime(ifModifiedSince); err != nil {
			return nil, fmt.Errorf("invalid If-Modified-Since HTTP-date: %w", err)
		}
		headers["If-Modified-Since"] = ifModifiedSince
	}
	for _, raw := range extraHeaders {
		name, value, err := parseHeaderFlag(raw)
		if err != nil {
			return nil, err
		}
		headers[name] = value
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return headers, nil
}

func parseHeaderFlag(raw string) (string, string, error) {
	// Accept both common CLI styles, while still canonicalizing the final
	// field name through net/http before the request is built.
	name, value, ok := strings.Cut(raw, ":")
	if !ok {
		name, value, ok = strings.Cut(raw, "=")
	}
	if !ok {
		return "", "", fmt.Errorf("invalid header %q: use 'Name: value' or 'Name=value'", raw)
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if name == "" {
		return "", "", fmt.Errorf("invalid header %q: header name is empty", raw)
	}
	if strings.ContainsAny(name, " \t\r\n:") {
		return "", "", fmt.Errorf("invalid header name %q", name)
	}
	return http.CanonicalHeaderKey(name), value, nil
}
