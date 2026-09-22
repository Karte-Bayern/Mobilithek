# Data Sources

This package converts more than one kind of upstream data into the same
generic `Event` / GeoJSON model, so different sources can be viewed with the
same MapLibre demo or combined on one map.

| Source | Client | Needs credentials | CLI |
| --- | --- | --- | --- |
| Mobilithek subscriptions (DATEX II XML) | `Client` | Yes: mTLS client certificate ([docs/certificates.md](certificates.md)) | `cmd/mobilithek-fetch`, `cmd/mobilithek-geojson` |
| Autobahn API (public JSON) | `AutobahnClient` | No | `cmd/mobilithek-autobahn` |

## Mobilithek Subscriptions

See [docs/endpoints.md](endpoints.md) and [docs/conversion.md](conversion.md).
Mobilithek is Germany's National Access Point; a subscription can mediate
data from many different upstream publishers, delivered as DATEX II XML.
`ExtractEventsFromDATEX2XML` is a generic extractor: it recognizes several
DATEX II record shapes (situation records of any subtype — maintenance
works, abnormal traffic, accidents, and so on — plus parking availability
and variable message sign records) but is not a full DATEX II domain
parser. See the "Important Limitation" note in docs/conversion.md.

## Autobahn API

`AutobahnClient` fetches the public, unauthenticated API behind the German
Autobahn-App (`https://verkehr.autobahn.de/o/autobahn`) directly in Go —
`examples/maplibre/data/build-real-roadworks.mjs` already used it from
Node.js to build the GitHub Pages demo excerpt; `AutobahnClient` makes the
same data available to any Go program using this package, for any road and
any of the API's service kinds:

- `roadworks`
- `warning`
- `closure`
- `electric_charging_station`
- `parking_lorryparkingfeatures`

```go
client, err := mobilithek.NewAutobahnClient()
events, err := client.FetchService(ctx, "A9", mobilithek.AutobahnRoadworks)
geojson := mobilithek.EventsToGeoJSON(events)
```

Or from the command line:

```bash
go run ./cmd/mobilithek-autobahn -roads A9,A92 -services roadworks,warning,closure -out out/autobahn.geojson
```

`-roads all` fetches every road the API currently lists; `-services all`
fetches every known service kind. Requests run with bounded concurrency
(`-concurrency`, default 4) so a broad selection doesn't hammer the public
API, and failed individual requests are logged as warnings rather than
aborting the whole run.

Every Autobahn API item becomes one `Event` with `Source: "autobahn-api"`,
`Road` set to the requested road ID, `Status` set to `"current"` or
`"future"`, and the service kind, blocked flag, subtitle, and route
recommendations carried in `Extra`.

## Combining Sources

Since both clients produce the same `Event`/GeoJSON model, `MergeGeoJSON`
combines their output (or output from any other source) into one
FeatureCollection with a recomputed bbox:

```go
mobilithekGeoJSON, _ := mobilithek.GeoJSONFromDATEX2XML(subscriptionXML, "subscription-123")
autobahnEvents, _ := autobahnClient.FetchServices(ctx, "A9", mobilithek.AllAutobahnServiceKinds())
autobahnGeoJSON := mobilithek.EventsToGeoJSON(autobahnEvents)

combined := mobilithek.MergeGeoJSON(mobilithekGeoJSON, autobahnGeoJSON)
```

`FilterEventsByBBox`, `FilterEventsByStatus`, and `FilterEventsBySource` (in
[filter.go](../filter.go)) narrow an `[]Event` slice — by geography, by
current/future, or back down to a single origin — before converting to
GeoJSON. `cmd/mobilithek-geojson` exposes the first two as `-bbox` and
`-status` flags.

## Adding Another Source

A new source only needs a function that produces `[]mobilithek.Event`;
everything downstream (GeoJSON conversion, filtering, merging, the MapLibre
demo) already works against that generic model. `autobahn.go` is a
reasonably small example to follow: an HTTP client, one struct that mirrors
the upstream JSON/XML shape, and a conversion function into `Event`.
