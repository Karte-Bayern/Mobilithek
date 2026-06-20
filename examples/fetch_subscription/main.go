package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/karte-bayern/mobilithek"
)

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

	response, err := client.FetchSubscription(
		ctx,
		os.Getenv("MOBILITHEK_SUBSCRIPTION_ID"),
		mobilithek.EndpointAuto,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("status=%d bytes=%d content-type=%s", response.StatusCode, len(response.Body), response.ContentType())
}
