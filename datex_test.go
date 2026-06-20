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
