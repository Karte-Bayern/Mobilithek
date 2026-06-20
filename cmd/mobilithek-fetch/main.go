package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/karte-bayern/mobilithek"
)

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

	response, err := client.FetchSubscription(ctx, *subscriptionID, mobilithek.EndpointKind(*endpoint))
	if err != nil {
		log.Fatal(err)
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
