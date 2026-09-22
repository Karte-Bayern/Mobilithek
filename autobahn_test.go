package mobilithek

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAutobahnFetchServiceLineStringRoadworks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/A9/services/roadworks" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roadworks":[{
			"identifier":"2026-1",
			"isBlocked":"false",
			"future":true,
			"point":"48.38701909199113,11.597210139249096",
			"display_type":"SHORT_TERM_ROADWORKS",
			"subtitle":" Nürnberg -> München",
			"title":"A9 | Allershausen - Fürholzen (West)",
			"description":["Line 1","","Line 2"],
			"routeRecommendation":[],
			"geometry":{"type":"LineString","coordinates":[[11.597210139,48.387019092],[11.5971938,48.385321901]]}
		}]}`))
	}))
	defer server.Close()

	client, err := NewAutobahnClient(WithAutobahnBaseURL(server.URL), WithAutobahnMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	events, err := client.FetchService(context.Background(), "A9", AutobahnRoadworks)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("FetchService() returned %d events, want 1", len(events))
	}

	event := events[0]
	if event.Source != "autobahn-api" {
		t.Fatalf("Source = %q, want autobahn-api", event.Source)
	}
	if event.Road != "A9" {
		t.Fatalf("Road = %q, want A9", event.Road)
	}
	if event.Status != "future" {
		t.Fatalf("Status = %q, want future", event.Status)
	}
	if len(event.Coordinates) != 2 {
		t.Fatalf("len(Coordinates) = %d, want 2 (from geometry, not the point fallback)", len(event.Coordinates))
	}
	if event.Coordinates[0].Lat != 48.387019092 || event.Coordinates[0].Lon != 11.597210139 {
		t.Fatalf("Coordinates[0] = %+v, want lat/lon swapped from [lon,lat] geometry order", event.Coordinates[0])
	}
	if event.Description != "Line 1 Line 2" {
		t.Fatalf("Description = %q, want blank lines dropped", event.Description)
	}
}

func TestAutobahnFetchServicePointOnlyFallsBackToCoordinateField(t *testing.T) {
	// Regression test: the Autobahn API's "point" string field does not use a
	// fixed lat,lon order (it mirrors whatever order "coordinate"'s keys
	// happen to serialize in, which differs per service kind), so this
	// checks that the string-typed "coordinate" object - as returned by
	// electric_charging_station - is still parsed correctly rather than by
	// position.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"electric_charging_station":[{
			"identifier":"29963",
			"isBlocked":"false",
			"future":false,
			"point":"11.598854,49.893506",
			"coordinate":{"long":"11.598854","lat":"49.893506"},
			"display_type":"STRONG_ELECTRIC_CHARGING_STATION",
			"title":"A9 | München | Sophienberg W",
			"description":["4 Ladepunkte"]
		}]}`))
	}))
	defer server.Close()

	client, err := NewAutobahnClient(WithAutobahnBaseURL(server.URL), WithAutobahnMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	events, err := client.FetchService(context.Background(), "A9", AutobahnElectricChargingStation)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("FetchService() returned %d events, want 1", len(events))
	}
	coordinates := events[0].Coordinates
	if len(coordinates) != 1 {
		t.Fatalf("len(Coordinates) = %d, want 1 (from the coordinate fallback)", len(coordinates))
	}
	if coordinates[0].Lat != 49.893506 || coordinates[0].Lon != 11.598854 {
		t.Fatalf("Coordinates[0] = %+v, want {Lat:49.893506 Lon:11.598854} from the named coordinate.lat/long fields", coordinates[0])
	}
	if events[0].Status != "current" {
		t.Fatalf("Status = %q, want current", events[0].Status)
	}
}

func TestAutobahnFetchServiceNumericCoordinateField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"roadworks":[{
			"identifier":"r1",
			"future":false,
			"coordinate":{"lat":48.387019,"long":11.597210},
			"title":"A9 test"
		}]}`))
	}))
	defer server.Close()

	client, err := NewAutobahnClient(WithAutobahnBaseURL(server.URL), WithAutobahnMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	events, err := client.FetchService(context.Background(), "A9", AutobahnRoadworks)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || len(events[0].Coordinates) != 1 {
		t.Fatalf("FetchService() = %+v, want exactly 1 event with 1 coordinate", events)
	}
	if events[0].Coordinates[0].Lat != 48.387019 || events[0].Coordinates[0].Lon != 11.597210 {
		t.Fatalf("Coordinates[0] = %+v, want {Lat:48.387019 Lon:11.597210}", events[0].Coordinates[0])
	}
}

func TestAutobahnFetchServiceUnknownKindReturnsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"warning":[]}`))
	}))
	defer server.Close()

	client, err := NewAutobahnClient(WithAutobahnBaseURL(server.URL), WithAutobahnMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	events, err := client.FetchService(context.Background(), "A9", AutobahnWarning)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("FetchService() returned %d events, want 0", len(events))
	}
}

func TestAutobahnFetchServiceRejectsMissingArguments(t *testing.T) {
	client, err := NewAutobahnClient()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.FetchService(context.Background(), "", AutobahnWarning); err == nil {
		t.Fatal("FetchService() accepted an empty road ID")
	}
	if _, err := client.FetchService(context.Background(), "A9", ""); err == nil {
		t.Fatal("FetchService() accepted an empty service kind")
	}
}

func TestAutobahnFetchServiceErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewAutobahnClient(WithAutobahnBaseURL(server.URL), WithAutobahnMaxRetries(0))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.FetchService(context.Background(), "A9", AutobahnWarning)
	if err == nil {
		t.Fatal("FetchService() did not return an error for HTTP 404")
	}
	statusErr, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("error type = %T, want *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404", statusErr.StatusCode)
	}
}

func TestAutobahnGetRetriesOnServerError(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"warning":[]}`))
	}))
	defer server.Close()

	client, err := NewAutobahnClient(
		WithAutobahnBaseURL(server.URL),
		WithAutobahnMaxRetries(3),
		WithAutobahnRetryBackoff(1),
	)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.FetchService(context.Background(), "A9", AutobahnWarning); err != nil {
		t.Fatalf("FetchService() returned an error after the server recovered: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("server received %d attempts, want 3", attempts)
	}
}

func TestAutobahnRoads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"roads":["A1","A9","A92"]}`))
	}))
	defer server.Close()

	client, err := NewAutobahnClient(WithAutobahnBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	roads, err := client.Roads(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roads) != 3 || roads[1] != "A9" {
		t.Fatalf("Roads() = %v, want [A1 A9 A92]", roads)
	}
}

func TestDecodeAutobahnGeometryDropsInvalidCoordinates(t *testing.T) {
	geometry := &autobahnGeometry{
		Type:        "LineString",
		Coordinates: []byte(`[[11.4,48.1],[200,91]]`),
	}
	coordinates := decodeAutobahnGeometry(geometry)
	if len(coordinates) != 1 {
		t.Fatalf("decodeAutobahnGeometry() returned %d coordinates, want 1", len(coordinates))
	}
}

func TestAutobahnFloatUnmarshalsStringAndNumber(t *testing.T) {
	var fromString autobahnFloat
	if err := fromString.UnmarshalJSON([]byte(`"49.893506"`)); err != nil {
		t.Fatal(err)
	}
	if float64(fromString) != 49.893506 {
		t.Fatalf("UnmarshalJSON(string) = %v, want 49.893506", fromString)
	}

	var fromNumber autobahnFloat
	if err := fromNumber.UnmarshalJSON([]byte(`48.387019`)); err != nil {
		t.Fatal(err)
	}
	if float64(fromNumber) != 48.387019 {
		t.Fatalf("UnmarshalJSON(number) = %v, want 48.387019", fromNumber)
	}

	var fromInvalid autobahnFloat
	if err := fromInvalid.UnmarshalJSON([]byte(`"not-a-number"`)); err == nil {
		t.Fatal("UnmarshalJSON() accepted a non-numeric string")
	}
}
