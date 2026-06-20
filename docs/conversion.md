# XML To GeoJSON Conversion

Mobilithek subscription endpoints commonly return DATEX II XML. Browser map libraries such as MapLibre GL JS consume GeoJSON more directly.

The intended demo workflow is:

1. Fetch `subscription.xml`.
2. Convert that XML to `events.geojson`.
3. Open the MapLibre demo against the GeoJSON file.

## Fetch And Convert In One Step

```bash
export MOBILITHEK_SUBSCRIPTION_ID=123456789012345678
export MOBILITHEK_CERT_FILE=certs/client.crt
export MOBILITHEK_KEY_FILE=certs/client.key

go run ./cmd/mobilithek-fetch \
  -out out/subscription.xml \
  -geojson-out out/events.geojson
```

## Convert An Existing XML File

```bash
go run ./cmd/mobilithek-geojson \
  -in out/subscription.xml \
  -out out/events.geojson
```

## Test Without Credentials

The repository includes a tiny synthetic XML fixture:

```bash
go run ./cmd/mobilithek-geojson \
  -in examples/data/sample-subscription.xml \
  -out out/events.geojson
```

## Open Converted Data In The MapLibre Demo

```bash
go run ./examples/maplibre
```

Then open:

```text
http://127.0.0.1:8787/?data=/converted/events.geojson
```

## Important Limitation

The converter is intentionally generic. It extracts common event metadata and display coordinates from DATEX II-like XML, but it is not a full DATEX II domain parser. Production projects should add source-specific normalization for road references, linear references, Alert-C, TPEG, and other richer DATEX II location models.
