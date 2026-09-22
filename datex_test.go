package mobilithek

import (
	"os"
	"testing"
)

func TestGeoJSONFromDATEX2XML(t *testing.T) {
	xml := []byte(`
		<d2LogicalModel>
			<situationRecord id="sample-1" xsi:type="MaintenanceWorks" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
				<generalPublicComment>
					<comment><values><value>Roadworks near Munich</value></values></comment>
				</generalPublicComment>
				<overallStartTime>2026-06-19T08:00:00Z</overallStartTime>
				<overallEndTime>2026-06-30T18:00:00Z</overallEndTime>
				<locationForDisplay>
					<latitude>48.1552</latitude>
					<longitude>11.4536</longitude>
				</locationForDisplay>
				<locationForDisplay>
					<latitude>48.1731</latitude>
					<longitude>11.5068</longitude>
				</locationForDisplay>
			</situationRecord>
		</d2LogicalModel>
	`)

	geojson, err := GeoJSONFromDATEX2XML(xml, "test-source")
	if err != nil {
		t.Fatal(err)
	}
	if len(geojson.Features) != 1 {
		t.Fatalf("GeoJSONFromDATEX2XML() produced %d features, want 1", len(geojson.Features))
	}

	feature := geojson.Features[0]
	if feature.Geometry.Type != "LineString" {
		t.Fatalf("Geometry.Type = %q, want LineString", feature.Geometry.Type)
	}
	if feature.Properties["title"] != "Roadworks near Munich" {
		t.Fatalf("title = %q, want Roadworks near Munich", feature.Properties["title"])
	}
}

func TestExtractEventsFromDATEX2XMLParkingRecord(t *testing.T) {
	xml := []byte(`
		<d2LogicalModel>
			<parkingRecordStatus id="parking-1">
				<parkingFacilityStatus>spacesAvailable</parkingFacilityStatus>
				<parkingNumberOfVacantSpaces>42</parkingNumberOfVacantSpaces>
				<totalNumberOfVacantSpaces>120</totalNumberOfVacantSpaces>
				<locationForDisplay>
					<latitude>48.1352</latitude>
					<longitude>11.5836</longitude>
				</locationForDisplay>
			</parkingRecordStatus>
		</d2LogicalModel>
	`)

	events, err := ExtractEventsFromDATEX2XML(xml, "test-source")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("ExtractEventsFromDATEX2XML() produced %d events, want 1", len(events))
	}

	event := events[0]
	if event.Type != "spacesAvailable" {
		t.Fatalf("Type = %q, want spacesAvailable", event.Type)
	}
	if got := event.Extra["parkingnumberofvacantspaces"]; len(got) != 1 || got[0] != "42" {
		t.Fatalf("Extra[parkingnumberofvacantspaces] = %v, want [42]", got)
	}
	if got := event.Extra["totalnumberofvacantspaces"]; len(got) != 1 || got[0] != "120" {
		t.Fatalf("Extra[totalnumberofvacantspaces] = %v, want [120]", got)
	}
	if len(event.Coordinates) != 1 {
		t.Fatalf("len(Coordinates) = %d, want 1", len(event.Coordinates))
	}
}

func TestExtractEventsFromDATEX2XMLVmsUnit(t *testing.T) {
	xml := []byte(`
		<d2LogicalModel>
			<vmsUnit id="vms-1">
				<vmsText>
					<textPage>
						<text>
							<values>
								<value>Stau 3km</value>
							</values>
						</text>
					</textPage>
				</vmsText>
				<locationForDisplay>
					<latitude>48.2</latitude>
					<longitude>11.6</longitude>
				</locationForDisplay>
			</vmsUnit>
		</d2LogicalModel>
	`)

	events, err := ExtractEventsFromDATEX2XML(xml, "test-source")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("ExtractEventsFromDATEX2XML() produced %d events, want 1", len(events))
	}
	if got := events[0].Description; got != "Stau 3km" {
		t.Fatalf("Description = %q, want %q", got, "Stau 3km")
	}
}

func TestExtractEventsFromDATEX2XMLResetsPendingCoordinateOnNestedEventStart(t *testing.T) {
	// Regression test: a stray half-coordinate belonging to an abandoned
	// outer event must not leak into a nested inner event's coordinates.
	xml := []byte(`
		<situationRecord id="outer">
			<longitude>40.0</longitude>
			<situationRecord id="inner">
				<latitude>20.0</latitude>
			</situationRecord>
		</situationRecord>
	`)

	events, err := ExtractEventsFromDATEX2XML(xml, "test-source")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("ExtractEventsFromDATEX2XML() produced %d events, want 2", len(events))
	}

	var inner *Event
	for i := range events {
		if events[i].ID == "inner" {
			inner = &events[i]
		}
	}
	if inner == nil {
		t.Fatal("no event with id=inner in the result")
	}
	if len(inner.Coordinates) != 0 {
		t.Fatalf("inner.Coordinates = %+v, want empty (its own latitude must not pair with the outer event's stray longitude)", inner.Coordinates)
	}
}

func TestUniqueCoordinatesEnforcesLimit(t *testing.T) {
	input := make([]Coordinate, 0, 300)
	for i := 0; i < 300; i++ {
		input = append(input, Coordinate{Lat: 48.0 + float64(i)*0.0001, Lon: 11.0})
	}

	got := uniqueCoordinates(input, 250)
	if len(got) != 250 {
		t.Fatalf("uniqueCoordinates() returned %d coordinates, want the 250-item cap to apply", len(got))
	}
}

func TestSampleSubscriptionFixtureConverts(t *testing.T) {
	body, err := os.ReadFile("examples/data/sample-subscription.xml")
	if err != nil {
		t.Fatal(err)
	}

	geojson, err := GeoJSONFromDATEX2XML(body, "sample-subscription.xml")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(geojson.Features), 2; got != want {
		t.Fatalf("sample fixture produced %d features, want %d", got, want)
	}
}

func TestEventsToGeoJSONAddsRFC7946BBoxesAndSkipsInvalidCoordinates(t *testing.T) {
	geojson := EventsToGeoJSON([]Event{
		{
			ID: "line",
			Coordinates: []Coordinate{
				{Lat: 48.1, Lon: 11.4},
				{Lat: 48.3, Lon: 11.8},
				{Lat: 91, Lon: 11.9},
			},
		},
		{
			ID:          "invalid",
			Coordinates: []Coordinate{{Lat: 0, Lon: 0}},
		},
	})

	if got, want := len(geojson.Features), 1; got != want {
		t.Fatalf("EventsToGeoJSON() produced %d features, want %d", got, want)
	}
	assertFloatSlice(t, geojson.BBox, []float64{11.4, 48.1, 11.8, 48.3})
	assertFloatSlice(t, geojson.Features[0].BBox, []float64{11.4, 48.1, 11.8, 48.3})
}

func assertFloatSlice(t *testing.T, got []float64, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len(%v) = %d, want %d", got, len(got), len(want))
	}
	for index := range got {
		if got[index] != want[index] {
			t.Fatalf("value[%d] = %f, want %f in %v", index, got[index], want[index], got)
		}
	}
}
