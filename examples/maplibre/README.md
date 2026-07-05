# MapLibre Example

This browser example demonstrates how a frontend can consume a prepared GeoJSON event file.

It uses a static excerpt of 42 real roadworks from `data/real-roadworks.geojson` by default. That makes the GitHub Pages demo useful without publishing a full upstream export or requiring Mobilithek credentials.

## Run

From the repository root:

```bash
go run ./examples/maplibre
```

Open:

```text
http://127.0.0.1:8787/
```

## Use Converted Subscription Data

After fetching a real subscription and converting it to `out/events.geojson`, open:

```text
http://127.0.0.1:8787/?data=/converted/events.geojson
```

The example server exposes only that exact converted file from `out/`.

To test this conversion flow without Mobilithek credentials, create `out/events.geojson` from the included synthetic XML:

```bash
go run ./cmd/mobilithek-geojson \
  -in examples/data/sample-subscription.xml \
  -out out/events.geojson
```

## Notes

- The example loads MapLibre GL JS from a CDN.
- `data/real-roadworks.geojson` is intentionally small, static, and simplified for GitHub Pages.
- Refresh the real excerpt with `make real-sample` from the repository root.
- `data/sample-events.geojson` remains available as a synthetic fixture.
- This demo does not require Mobilithek credentials.
