# mobilithek

Go client and small demos for retrieving Mobilithek subscriptions, including certificate-authenticated machine-account access.

This repository is intended for two use cases:

- as an importable Go package: `github.com/karte-bayern/mobilithek`
- as a minimal public demo for fetching Mobilithek data and displaying prepared GeoJSON in MapLibre GL JS

No credentials, private keys, real certificates, real subscription IDs, or production data are included.

## Install

```bash
go get github.com/karte-bayern/mobilithek
```

## Quick Start With Make

The shortest offline demo works without Mobilithek credentials:

```bash
make sample web
```

That converts the synthetic XML fixture from `examples/data/sample-subscription.xml` to `out/events.geojson` and starts the MapLibre demo. Open:

```text
http://127.0.0.1:8787/?data=/converted/events.geojson
```

For real Mobilithek data, put your `.p12`/`.pfx` certificate bundle anywhere below this repository, then run:

```bash
make cert
MOBILITHEK_SUBSCRIPTION_ID=123456789012345678 make fetch
make web
```

`make cert` searches this directory and all subdirectories for a `.p12`/`.pfx` file, converts it to `certs/client.crt` and `certs/client.key`, and keeps the private key local. If no bundle is found, it tells you where to get the certificate in Mobilithek and asks for a path.

`make fetch` downloads the subscription XML to `out/subscription.xml` and writes the converted GeoJSON to `out/events.geojson`.

`make web` starts the MapLibre demo. Open the converted data view:

```text
http://127.0.0.1:8787/?data=/converted/events.geojson
```

After the certificate has been converted, the complete real-data flow can be run as:

```bash
MOBILITHEK_SUBSCRIPTION_ID=123456789012345678 make all
```

Run `make` to list all available commands.

## Fetch A Subscription

Mobilithek machine accounts usually provide a PKCS#12 file (`.p12`/`.pfx`) and a password. The Go standard library expects PEM files, so convert the certificate first:

```bash
openssl pkcs12 -in certs/client.p12 -clcerts -nokeys -out certs/client.crt
openssl pkcs12 -in certs/client.p12 -nocerts -nodes -out certs/client.key
chmod 600 certs/client.key
```

Then fetch a subscription with the example CLI:

```bash
export MOBILITHEK_SUBSCRIPTION_ID=123456789012345678
export MOBILITHEK_CERT_FILE=certs/client.crt
export MOBILITHEK_KEY_FILE=certs/client.key

go run ./cmd/mobilithek-fetch \
  -out out/subscription.xml \
  -geojson-out out/events.geojson
```

`123456789012345678` is a placeholder. Use your own subscription ID from your Mobilithek account.

The CLI writes the raw DATEX II XML to `out/subscription.xml` and, when `-geojson-out` is set, a generic event GeoJSON file to `out/events.geojson`.

## Use As A Go Package

```go
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

	log.Printf("status=%d bytes=%d url=%s", response.StatusCode, len(response.Body), response.URL)
}
```

## MapLibre Demo

The MapLibre example intentionally uses a tiny synthetic GeoJSON file by default. It demonstrates the frontend pattern without publishing subscription data and without requiring credentials.

```bash
go run ./examples/maplibre
```

Open:

```text
http://127.0.0.1:8787/
```

See [examples/maplibre/README.md](examples/maplibre/README.md).

You can also test the converter without credentials:

```bash
go run ./cmd/mobilithek-geojson \
  -in examples/data/sample-subscription.xml \
  -out out/events.geojson
```

Then open the converted output in the map:

```text
http://127.0.0.1:8787/?data=/converted/events.geojson
```

To view converted subscription data after running the fetch command above, open:

```text
http://127.0.0.1:8787/?data=/converted/events.geojson
```

See [docs/conversion.md](docs/conversion.md) for the XML-to-GeoJSON workflow and limitations.

## Repository Layout

```text
.
├── Makefile                   One-command local workflows
├── cmd/mobilithek-fetch/      Small CLI for manual subscription fetches
├── cmd/mobilithek-geojson/    Converts fetched DATEX II XML to GeoJSON
├── docs/                      Operational notes
├── examples/data/             Synthetic XML fixture for converter testing
├── examples/fetch_subscription/
├── examples/maplibre/         Browser demo with synthetic GeoJSON
├── client.go                  HTTP and TLS client
├── datex.go                   Generic DATEX II event extraction
├── endpoints.go               Mobilithek endpoint URL helpers
├── geojson.go                 GeoJSON conversion helpers
└── response.go                Response helpers
```

## Security Notes

- Never commit `.p12`, `.pfx`, `.key`, `.pem`, `.crt`, `.env`, or generated production `data/`.
- Use environment variables or a local secret manager for credential paths.
- Keep private keys readable only by the local user, for example `chmod 600 certs/client.key`.
- The included `.gitignore` excludes the common secret and output file patterns.

## License

Add a license before publishing if this package should be reusable by third parties.
