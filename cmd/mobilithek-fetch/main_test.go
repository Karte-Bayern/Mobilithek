package main

import "testing"

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
