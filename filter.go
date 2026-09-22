package mobilithek

import "strings"

// BBox is a geographic bounding box in [west, south, east, north] (WGS84
// degrees) order, matching the RFC 7946 bbox member order used elsewhere in
// this package.
type BBox struct {
	West  float64
	South float64
	East  float64
	North float64
}

// Contains reports whether the given coordinate falls inside the box.
func (b BBox) Contains(lat float64, lon float64) bool {
	return lon >= b.West && lon <= b.East && lat >= b.South && lat <= b.North
}

// Intersects reports whether any of the event's coordinates fall inside bbox.
func (e Event) Intersects(bbox BBox) bool {
	for _, coordinate := range e.Coordinates {
		if bbox.Contains(coordinate.Lat, coordinate.Lon) {
			return true
		}
	}
	return false
}

// FilterEventsByBBox keeps only events with at least one coordinate inside
// bbox. Events without coordinates are dropped.
func FilterEventsByBBox(events []Event, bbox BBox) []Event {
	output := make([]Event, 0, len(events))
	for _, event := range events {
		if event.Intersects(bbox) {
			output = append(output, event)
		}
	}
	return output
}

// FilterEventsByStatus keeps only events whose Status matches one of
// statuses (case-insensitive). An empty statuses list returns events
// unchanged.
func FilterEventsByStatus(events []Event, statuses ...string) []Event {
	if len(statuses) == 0 {
		return events
	}
	allowed := allowedSet(statuses)
	output := make([]Event, 0, len(events))
	for _, event := range events {
		if allowed[strings.ToLower(event.Status)] {
			output = append(output, event)
		}
	}
	return output
}

// FilterEventsBySource keeps only events whose Source matches one of
// sources (case-insensitive). An empty sources list returns events
// unchanged. This is what lets a caller combine events from Mobilithek and
// the Autobahn API and later split or filter them back out by origin.
func FilterEventsBySource(events []Event, sources ...string) []Event {
	if len(sources) == 0 {
		return events
	}
	allowed := allowedSet(sources)
	output := make([]Event, 0, len(events))
	for _, event := range events {
		if allowed[strings.ToLower(event.Source)] {
			output = append(output, event)
		}
	}
	return output
}

func allowedSet(values []string) map[string]bool {
	allowed := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value != "" {
			allowed[value] = true
		}
	}
	return allowed
}

// MergeGeoJSON combines multiple FeatureCollections, produced by
// EventsToGeoJSON from any combination of sources (Mobilithek subscriptions,
// the Autobahn API, or others), into one FeatureCollection with a
// recomputed collection bbox.
//
// The collection bbox is derived only from features that already carry a
// per-feature bbox member, which EventsToGeoJSON always sets; a feature from
// some other origin that omits bbox still merges in but does not contribute
// to the recomputed bounds.
func MergeGeoJSON(collections ...GeoJSON) GeoJSON {
	merged := GeoJSON{Type: "FeatureCollection", Features: []GeoJSONFeature{}}
	for _, collection := range collections {
		merged.Features = append(merged.Features, collection.Features...)
	}

	var bounds coordinateBounds
	for _, feature := range merged.Features {
		if len(feature.BBox) != 4 {
			continue
		}
		bounds.extend(Coordinate{Lon: feature.BBox[0], Lat: feature.BBox[1]})
		bounds.extend(Coordinate{Lon: feature.BBox[2], Lat: feature.BBox[3]})
	}
	merged.BBox = bounds.bbox()

	return merged
}
