package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/karte-bayern/mobilithek"
)

// Minimal end-to-end usage of the package: fetch a subscription and convert
// it to GeoJSON. See ./cmd/mobilithek-fetch for a full-featured CLI with
// flags, conditional requests, and stdout support.
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client, err := mobilithek.New(
		mobilithek.WithClientCertificate(
			os.Getenv("MOBILITHEK_CERT_FILE"),
			os.Getenv("MOBILITHEK_KEY_FILE"),
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	subscriptionID := os.Getenv("MOBILITHEK_SUBSCRIPTION_ID")
	response, err := client.FetchSubscription(ctx, subscriptionID, mobilithek.EndpointAuto)
	if err != nil {
		log.Fatal(err)
	}
	if !response.OK() {
		log.Fatalf("HTTP %d from %s", response.StatusCode, response.URL)
	}
	log.Printf("status=%d bytes=%d content-type=%s", response.StatusCode, len(response.Body), response.ContentType())

	geojson, err := mobilithek.GeoJSONFromDATEX2XML(response.Body, "subscription-"+subscriptionID)
	if err != nil {
		log.Fatal(err)
	}
	output, err := mobilithek.MarshalGeoJSON(geojson, true)
	if err != nil {
		log.Fatal(err)
	}

	if err := os.MkdirAll("out", 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("out/events.geojson", output, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote out/events.geojson (%d features)", len(geojson.Features))
}
