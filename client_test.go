package mobilithek

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRejectsIncompleteClientCertificate(t *testing.T) {
	_, err := New(WithClientCertificate("certs/client.crt", ""))
	if err == nil {
		t.Fatal("New() accepted an incomplete client certificate")
	}
}

func TestReadLimitedRejectsOversize(t *testing.T) {
	_, err := readLimited(strings.NewReader("abcdef"), 3)
	if err == nil {
		t.Fatal("readLimited() accepted oversized response")
	}
}

func TestResponseContentType(t *testing.T) {
	response := &Response{
		Header: http.Header{
			"Content-Type": []string{"text/xml; charset=utf-8"},
		},
	}
	if got := response.ContentType(); got != "text/xml" {
		t.Fatalf("ContentType() = %q, want text/xml", got)
	}
}

func TestResponseValidators(t *testing.T) {
	header := http.Header{}
	header.Set("ETag", `"abc123"`)
	header.Set("Last-Modified", "Sun, 05 Jul 2026 12:00:00 GMT")
	response := &Response{Header: header}
	if got := response.ETag(); got != `"abc123"` {
		t.Fatalf("ETag() = %q, want %q", got, `"abc123"`)
	}
	if _, ok := response.LastModifiedTime(); !ok {
		t.Fatal("LastModifiedTime() did not parse a valid HTTP-date")
	}
}

func TestFetchSubscriptionWithHeaders(t *testing.T) {
	var gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIfNoneMatch = r.Header.Get("If-None-Match")
		w.WriteHeader(http.StatusNotModified)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.FetchSubscriptionWithHeaders(
		context.Background(),
		"123",
		EndpointGeneric,
		map[string]string{"If-None-Match": `"known"`},
	)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusNotModified {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusNotModified)
	}
	if gotIfNoneMatch != `"known"` {
		t.Fatalf("If-None-Match = %q, want %q", gotIfNoneMatch, `"known"`)
	}
}
