package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestRequestHeaders(t *testing.T) {
	headers, err := requestHeaders(
		`"etag-value"`,
		"Sun, 05 Jul 2026 12:00:00 GMT",
		[]string{"accept-language: de-DE", "x-demo=value"},
	)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{
		"If-None-Match":     `"etag-value"`,
		"If-Modified-Since": "Sun, 05 Jul 2026 12:00:00 GMT",
		"Accept-Language":   "de-DE",
		"X-Demo":            "value",
	}
	for key, value := range want {
		if headers[key] != value {
			t.Fatalf("headers[%q] = %q, want %q", key, headers[key], value)
		}
	}
}

func TestRequestHeadersRejectsInvalidHTTPDate(t *testing.T) {
	if _, err := requestHeaders("", "2026-07-05", nil); err == nil {
		t.Fatal("requestHeaders accepted a non-HTTP-date If-Modified-Since value")
	}
}

func TestParseHeaderFlagRejectsInvalidHeader(t *testing.T) {
	if _, _, err := parseHeaderFlag("bad header: value"); err == nil {
		t.Fatal("parseHeaderFlag accepted an invalid header name")
	}
}

func TestParseHeaderFlagEqualsValueContainingColon(t *testing.T) {
	// Regression test: "Name=value" is a documented form, and its value may
	// itself contain a colon (e.g. a time like "12:30pm"). That must not be
	// misparsed as "Name=12" with a bogus header name.
	name, value, err := parseHeaderFlag("X-Demo=12:30pm")
	if err != nil {
		t.Fatal(err)
	}
	if name != "X-Demo" || value != "12:30pm" {
		t.Fatalf("parseHeaderFlag() = (%q, %q), want (\"X-Demo\", \"12:30pm\")", name, value)
	}
}

func TestWriteGeoJSONDashWritesToStdoutInsteadOfALiteralFile(t *testing.T) {
	// Regression test: -geojson-out - must follow the same "-" means stdout
	// convention as -out, instead of creating a real file literally named "-".
	dir := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previous)

	originalStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	sampleXML := []byte(`<d2LogicalModel><situationRecord id="s1"><locationForDisplay><latitude>48.1</latitude><longitude>11.4</longitude></locationForDisplay></situationRecord></d2LogicalModel>`)
	writeErr := writeGeoJSON("-", sampleXML, "test")

	w.Close()
	os.Stdout = originalStdout
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "FeatureCollection") {
		t.Fatalf("stdout = %q, want it to contain the GeoJSON output", output)
	}
	if _, err := os.Stat("-"); err == nil {
		t.Fatal("writeGeoJSON(\"-\", ...) created a literal file named \"-\" instead of writing to stdout")
	}
}

func TestParseHeaderFlagBareColonWithoutSpace(t *testing.T) {
	name, value, err := parseHeaderFlag("X-Demo:value")
	if err != nil {
		t.Fatal(err)
	}
	if name != "X-Demo" || value != "value" {
		t.Fatalf("parseHeaderFlag() = (%q, %q), want (\"X-Demo\", \"value\")", name, value)
	}
}
