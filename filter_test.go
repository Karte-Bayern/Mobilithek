package mobilithek

import "testing"

func TestFilterEventsByBBox(t *testing.T) {
	munich := Event{ID: "munich", Coordinates: []Coordinate{{Lat: 48.14, Lon: 11.58}}}
	berlin := Event{ID: "berlin", Coordinates: []Coordinate{{Lat: 52.52, Lon: 13.40}}}
	noCoordinates := Event{ID: "no-coordinates"}

	bavaria := BBox{West: 8.8, South: 47.2, East: 13.9, North: 50.7}
	filtered := FilterEventsByBBox([]Event{munich, berlin, noCoordinates}, bavaria)

	if len(filtered) != 1 || filtered[0].ID != "munich" {
		t.Fatalf("FilterEventsByBBox() = %+v, want only munich", filtered)
	}
}

func TestFilterEventsByStatus(t *testing.T) {
	events := []Event{
		{ID: "a", Status: "current"},
		{ID: "b", Status: "future"},
		{ID: "c", Status: "Future"},
	}

	filtered := FilterEventsByStatus(events, "future")
	if len(filtered) != 2 {
		t.Fatalf("FilterEventsByStatus() returned %d events, want 2 (case-insensitive match)", len(filtered))
	}

	if got := FilterEventsByStatus(events); len(got) != 3 {
		t.Fatalf("FilterEventsByStatus() with no statuses returned %d events, want all 3 unchanged", len(got))
	}
}

func TestFilterEventsBySource(t *testing.T) {
	events := []Event{
		{ID: "a", Source: "autobahn-api"},
		{ID: "b", Source: "subscription-123"},
	}

	filtered := FilterEventsBySource(events, "autobahn-api")
	if len(filtered) != 1 || filtered[0].ID != "a" {
		t.Fatalf("FilterEventsBySource() = %+v, want only a", filtered)
	}
}

func TestMergeGeoJSONCombinesFeaturesAndRecomputesBBox(t *testing.T) {
	left := EventsToGeoJSON([]Event{{ID: "left", Coordinates: []Coordinate{{Lat: 48.1, Lon: 11.4}}}})
	right := EventsToGeoJSON([]Event{{ID: "right", Coordinates: []Coordinate{{Lat: 52.5, Lon: 13.4}}}})

	merged := MergeGeoJSON(left, right)
	if len(merged.Features) != 2 {
		t.Fatalf("MergeGeoJSON() produced %d features, want 2", len(merged.Features))
	}
	assertFloatSlice(t, merged.BBox, []float64{11.4, 48.1, 13.4, 52.5})
}

func TestMergeGeoJSONSkipsFeaturesWithoutBBoxInBoundsCalculation(t *testing.T) {
	withBBox := EventsToGeoJSON([]Event{{ID: "has-bbox", Coordinates: []Coordinate{{Lat: 48.1, Lon: 11.4}}}})
	withoutBBox := GeoJSON{
		Type: "FeatureCollection",
		Features: []GeoJSONFeature{{
			Type:       "Feature",
			Geometry:   GeoJSONGeometry{Type: "Point", Coordinates: []float64{100, 50}},
			Properties: map[string]interface{}{},
		}},
	}

	merged := MergeGeoJSON(withBBox, withoutBBox)
	if len(merged.Features) != 2 {
		t.Fatalf("MergeGeoJSON() produced %d features, want 2", len(merged.Features))
	}
	assertFloatSlice(t, merged.BBox, []float64{11.4, 48.1, 11.4, 48.1})
}
