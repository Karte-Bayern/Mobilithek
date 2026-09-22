package mobilithek

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

func TestFetchURLDecompressesGzipResponse(t *testing.T) {
	const want = "hello from a gzip response"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(want))
		_ = gz.Close()
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.FetchURL(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(response.Body); got != want {
		t.Fatalf("Body = %q, want %q (gzip response was not decompressed correctly)", got, want)
	}
}

func TestFetchURLRejectsGzipBombPastMaxBytes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		// The compressed body itself is well under maxBytes; only the
		// decompressed size exceeds it, exercising the second readLimited
		// call in readResponseBody (the one guarding against gzip bombs).
		_, _ = gz.Write([]byte(strings.Repeat("a", 10_000)))
		_ = gz.Close()
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL), WithMaxBytes(1000))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.FetchURL(context.Background(), server.URL, nil)
	if err == nil {
		t.Fatal("FetchURL() accepted a decompressed body larger than maxBytes")
	}
	if !strings.Contains(err.Error(), "exceeds max bytes") {
		t.Fatalf("error = %v, want an 'exceeds max bytes' error", err)
	}
}

func TestFetchSubscriptionWithHeadersAutoModeFallsThroughFailingCandidates(t *testing.T) {
	var requestedPaths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		if strings.Contains(r.URL.Path, "datexv3") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.FetchSubscriptionWithHeaders(context.Background(), "123", EndpointAuto, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", response.StatusCode)
	}
	if len(requestedPaths) < 2 {
		t.Fatalf("only %d candidate(s) were tried, want the loop to fall through failing ones: %v", len(requestedPaths), requestedPaths)
	}
}

func TestFetchSubscriptionWithHeadersAutoModeReturnsErrorWhenEveryCandidateFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.FetchSubscriptionWithHeaders(context.Background(), "123", EndpointAuto, nil)
	if err == nil {
		t.Fatal("FetchSubscriptionWithHeaders() did not return an error when every EndpointAuto candidate returned 404")
	}
	statusErr, ok := err.(*StatusError)
	if !ok {
		t.Fatalf("error type = %T, want *StatusError", err)
	}
	if statusErr.StatusCode != http.StatusNotFound {
		t.Fatalf("StatusCode = %d, want 404", statusErr.StatusCode)
	}
}

func TestNewHTTPClientLoadsValidClientCertificateAndCA(t *testing.T) {
	certFile, keyFile, caFile := writeTestCertFiles(t)

	client, err := New(WithClientCertificate(certFile, keyFile), WithRootCA(caFile))
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport type = %T, want *http.Transport", client.httpClient.Transport)
	}
	if len(transport.TLSClientConfig.Certificates) != 1 {
		t.Fatalf("len(Certificates) = %d, want 1 (the client cert/key pair was not loaded)", len(transport.TLSClientConfig.Certificates))
	}
	if transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("RootCAs is nil, want the CA file's pool to be attached")
	}
}

func TestNewRejectsMalformedCAFile(t *testing.T) {
	dir := t.TempDir()
	caFile := dir + "/ca.pem"
	if err := os.WriteFile(caFile, []byte("not a valid PEM certificate"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := New(WithRootCA(caFile)); err == nil {
		t.Fatal("New() accepted a root CA file with no valid PEM certificates")
	}
}

// writeTestCertFiles generates a minimal self-signed certificate/key pair
// and a matching CA file, purely to exercise newHTTPClient's success paths;
// it is never used to make a real connection.
func writeTestCertFiles(t *testing.T) (certFile string, keyFile string, caFile string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "mobilithek-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	certFile = dir + "/client.crt"
	keyFile = dir + "/client.key"
	caFile = dir + "/ca.pem"

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	return certFile, keyFile, caFile
}

func TestStatusErrorFormatsEmptyBody(t *testing.T) {
	err := &StatusError{URL: "https://example.test/x", StatusCode: 503}
	if got, want := err.Error(), "https://example.test/x returned HTTP 503"; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestStatusErrorTruncatesLongBody(t *testing.T) {
	err := &StatusError{URL: "https://example.test/x", StatusCode: 500, Body: []byte(strings.Repeat("a", 300))}
	got := err.Error()
	if !strings.HasSuffix(got, strings.Repeat("a", 240)+"...") {
		t.Fatalf("Error() = %q, want a 240-char body followed by ...", got)
	}
}

func TestStatusErrorTruncationIsUTF8Safe(t *testing.T) {
	// A multi-byte rune sitting right at the truncation boundary must not be
	// split into invalid UTF-8.
	body := strings.Repeat("a", 239) + strings.Repeat("ü", 20)
	err := &StatusError{URL: "https://example.test/x", StatusCode: 500, Body: []byte(body)}
	if !utf8.ValidString(err.Error()) {
		t.Fatalf("Error() produced invalid UTF-8: %q", err.Error())
	}
}

func TestResponseOK(t *testing.T) {
	var nilResponse *Response
	if nilResponse.OK() {
		t.Fatal("OK() on a nil *Response should be false, not panic-inducing true")
	}
	if (&Response{StatusCode: 200}).OK() != true {
		t.Fatal("OK() = false for status 200, want true")
	}
	if (&Response{StatusCode: 300}).OK() != false {
		t.Fatal("OK() = true for status 300, want false (300 is outside [200,300))")
	}
	if (&Response{StatusCode: 404}).OK() != false {
		t.Fatal("OK() = true for status 404, want false")
	}
}

func TestFetchURLDefaultsToNoRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.FetchURL(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("FetchURL() returned an error instead of the 503 response: %v", err)
	}
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", response.StatusCode)
	}
	if attempts != 1 {
		t.Fatalf("server received %d requests, want exactly 1 (retries must be opt-in)", attempts)
	}
}

func TestFetchURLRetriesOnServerErrorThenSucceeds(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL), WithMaxRetries(3), WithRetryBackoff(1))
	if err != nil {
		t.Fatal(err)
	}

	response, err := client.FetchURL(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want 200", response.StatusCode)
	}
	if attempts != 3 {
		t.Fatalf("server received %d requests, want 3", attempts)
	}
}

func TestFetchURLReturnsLastResponseAfterExhaustingRetries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client, err := New(WithBaseURL(server.URL), WithMaxRetries(2), WithRetryBackoff(1))
	if err != nil {
		t.Fatal(err)
	}

	// A response that keeps failing on every retry is still a real HTTP
	// response, not a transport error, so FetchURL must hand it back with a
	// nil error (matching its existing "caller inspects StatusCode"
	// contract) rather than swallowing it into a Go error.
	response, err := client.FetchURL(context.Background(), server.URL, nil)
	if err != nil {
		t.Fatalf("FetchURL() returned an error instead of the last 503 response: %v", err)
	}
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("StatusCode = %d, want 503", response.StatusCode)
	}
	if attempts != 3 {
		t.Fatalf("server received %d requests, want 3 (1 initial + 2 retries)", attempts)
	}
}

func TestFetchURLReturnsErrorWhenEveryAttemptIsATransportFailure(t *testing.T) {
	client, err := New(
		WithBaseURL("https://127.0.0.1:0"),
		WithMaxRetries(2),
		WithRetryBackoff(1),
		WithTimeout(500*time.Millisecond),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Port 0 never accepts connections, so every attempt fails at the
	// transport level with no HTTP response to fall back to; FetchURL must
	// surface that as an error instead of returning a nil *Response.
	_, err = client.FetchURL(context.Background(), "https://127.0.0.1:0", nil)
	if err == nil {
		t.Fatal("FetchURL() did not return an error when every attempt failed at the transport level")
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
