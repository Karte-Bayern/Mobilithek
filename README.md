# mobilithek

Go client and small demos for retrieving [Mobilithek](https://mobilithek.info/) subscriptions, including certificate-authenticated machine-account access, plus a second, credential-free source: the public [Autobahn API](https://verkehr.autobahn.de/o/autobahn).

Two uses:

- Go package: `github.com/karte-bayern/mobilithek`
- Demo: fetch data from either source and display it as GeoJSON in MapLibre GL JS

No credentials, private keys, certificates, real subscription IDs, or production data are included. Built while working on [Karte.Bayern](https://karte.bayern/); the reusable parts of that work — machine-account access, subscription endpoints, certificate handling, Autobahn API access, and conversion to a shared GeoJSON model — live here. See [docs/sources.md](docs/sources.md) for the sources this package supports and how to combine them.

## Install

```bash
go get github.com/karte-bayern/mobilithek
```

## Quick Start

Offline, no Mobilithek credentials needed:

```bash
make sample web
```

With real data, put your `.p12`/`.pfx` certificate bundle anywhere below this repository, then:

```bash
make cert
MOBILITHEK_SUBSCRIPTION_ID=123456789012345678 make fetch web
```

Both open the MapLibre demo at `http://127.0.0.1:8787/?data=/converted/events.geojson`. Run `make` with no arguments to list every target (`cert`, `fetch`, `convert`, `sample`, `real-sample`, `web`, `all`, `test`, `check`, `clean`, `doctor`, ...).

## Fetch A Subscription Manually

Mobilithek machine accounts provide a PKCS#12 file (`.p12`/`.pfx`). Convert it to PEM once:

```bash
openssl pkcs12 -in certs/client.p12 -clcerts -nokeys -out certs/client.crt
openssl pkcs12 -in certs/client.p12 -nocerts -nodes -out certs/client.key
chmod 600 certs/client.key
```

Then fetch and convert with the bundled CLI:

```bash
export MOBILITHEK_SUBSCRIPTION_ID=123456789012345678
export MOBILITHEK_CERT_FILE=certs/client.crt
export MOBILITHEK_KEY_FILE=certs/client.key

go run ./cmd/mobilithek-fetch -out out/subscription.xml -geojson-out out/events.geojson
```

`123456789012345678` is a placeholder; use your own subscription ID. Conditional requests (`-etag`, `-if-modified-since`, repeated `-header`) are supported, and HTTP `304 Not Modified` is treated as a no-op — run `go run ./cmd/mobilithek-fetch -h` for all flags. Programmatic callers can pass `mobilithek.WithMaxRetries` to `mobilithek.New` to retry transient network errors and HTTP 429/5xx responses with backoff.

## Fetch From The Autobahn API (No Credentials)

```bash
go run ./cmd/mobilithek-autobahn -roads A9,A92 -services roadworks,warning,closure -out out/autobahn.geojson
```

or `make autobahn`. See [docs/sources.md](docs/sources.md) for every service kind, combining this with Mobilithek data via `MergeGeoJSON`, and filtering events with `-bbox`/`-status` on `mobilithek-geojson`.

## Use As A Go Package

See [examples/fetch_subscription/main.go](examples/fetch_subscription/main.go) for a minimal runnable example that fetches a subscription and converts it to GeoJSON.

## MapLibre And GitHub Pages Demo

Live at <https://karte-bayern.github.io/Mobilithek/examples/maplibre/>, backed by a static, deliberately outdated excerpt of 42 real Autobahn-App roadworks: [examples/maplibre/data/real-roadworks.geojson](examples/maplibre/data/real-roadworks.geojson) (refresh with `make real-sample`).

Run it locally:

```bash
go run ./examples/maplibre
```

Open `http://127.0.0.1:8787/`. See [examples/maplibre/README.md](examples/maplibre/README.md) for the interface and [docs/conversion.md](docs/conversion.md) for the XML-to-GeoJSON workflow and its limitations.

Generated GeoJSON follows RFC 7946 coordinate order (`longitude, latitude`) with optional `bbox` members on the collection and each feature.

## Repository Layout

```text
.
├── Makefile                   One-command local workflows
├── cmd/mobilithek-fetch/      Small CLI for manual subscription fetches
├── cmd/mobilithek-geojson/    Converts fetched DATEX II XML to GeoJSON
├── cmd/mobilithek-autobahn/   Fetches the public Autobahn API to GeoJSON
├── docs/                      Operational notes
├── examples/data/             Synthetic XML fixture for converter testing
├── examples/fetch_subscription/ Minimal runnable fetch-and-convert example
├── examples/maplibre/         Browser demo and GitHub Pages entry point
├── examples/maplibre/data/    Small static demo data and data generator
├── client.go                  HTTP and TLS client (with optional retries)
├── autobahn.go                Autobahn API client and Event conversion
├── datex.go                   Generic DATEX II event extraction
├── endpoints.go               Mobilithek endpoint URL helpers
├── geojson.go                 GeoJSON conversion and MergeGeoJSON
├── filter.go                  Event filtering (bbox, status, source)
├── retry.go                   Shared backoff helper for both clients
└── response.go                Response helpers
```

## Security Notes

- Never commit `.p12`, `.pfx`, `.key`, `.pem`, `.crt`, `.env`, or generated production `data/`.
- Use environment variables or a local secret manager for credential paths.
- Keep private keys readable only by the local user, for example `chmod 600 certs/client.key`.
- The included `.gitignore` excludes the common secret and output file patterns.

## License

MIT. See [LICENSE](LICENSE).
