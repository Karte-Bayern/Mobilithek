package mobilithek

import (
	"net/http"
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
