# MapLibre Sample Data

`real-roadworks.geojson` is a static excerpt of 42 real roadworks from the public Autobahn-App API, fetched on 2026-07-05.

The file intentionally keeps a balanced selection of Bayern-adjacent Autobahn roadworks and simplified line geometries. It is meant for the GitHub Pages demo at `/Mobilithek/examples/maplibre/`, not as a complete mirror of the upstream feed.

Regenerate it from the live public API with:

```bash
make real-sample
```

For reproducible local testing with previously downloaded fixtures, run:

```bash
node examples/maplibre/data/build-real-roadworks.mjs \
  --fixture-dir /private/tmp \
  --fetched-at 2026-07-05 \
  --output examples/maplibre/data/real-roadworks.geojson
```

`sample-events.geojson` remains a synthetic fallback fixture for tests and local experiments.
