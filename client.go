package mobilithek

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultUserAgent = "github.com/karte-bayern/mobilithek"
const defaultMaxBytes = 250 * 1024 * 1024

// Client fetches Mobilithek subscription data over HTTP or mTLS.
type Client struct {
	baseURL      string
	httpClient   *http.Client
	userAgent    string
	accept       string
	maxBytes     int64
	maxRetries   int
	retryBackoff time.Duration
}

// Option configures a Client created by New.
type Option func(*clientConfig) error

type clientConfig struct {
	baseURL            string
	httpClient         *http.Client
	userAgent          string
	accept             string
	maxBytes           int64
	certFile           string
	keyFile            string
	caFile             string
	insecureSkipVerify bool
	timeout            time.Duration
	maxRetries         int
	retryBackoff       time.Duration
}

// New creates a Mobilithek client with conservative defaults.
func New(options ...Option) (*Client, error) {
	cfg := clientConfig{
		baseURL:      DefaultBaseURL,
		userAgent:    defaultUserAgent,
		accept:       "application/xml,text/xml,application/json,text/json,*/*",
		maxBytes:     defaultMaxBytes,
		timeout:      60 * time.Second,
		retryBackoff: 200 * time.Millisecond,
	}

	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		var err error
		httpClient, err = newHTTPClient(cfg)
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		baseURL:      strings.TrimRight(cfg.baseURL, "/"),
		httpClient:   httpClient,
		userAgent:    cfg.userAgent,
		accept:       cfg.accept,
		maxBytes:     cfg.maxBytes,
		maxRetries:   cfg.maxRetries,
		retryBackoff: cfg.retryBackoff,
	}, nil
}

// WithBaseURL overrides the Mobilithek base URL.
func WithBaseURL(baseURL string) Option {
	return func(cfg *clientConfig) error {
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			return errors.New("base URL must not be empty")
		}
		cfg.baseURL = baseURL
		return nil
	}
}

// WithHTTPClient uses a caller-provided HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(cfg *clientConfig) error {
		if httpClient == nil {
			return errors.New("HTTP client must not be nil")
		}
		cfg.httpClient = httpClient
		return nil
	}
}

// WithUserAgent overrides the User-Agent header sent by FetchURL.
func WithUserAgent(userAgent string) Option {
	return func(cfg *clientConfig) error {
		userAgent = strings.TrimSpace(userAgent)
		if userAgent == "" {
			return errors.New("user agent must not be empty")
		}
		cfg.userAgent = userAgent
		return nil
	}
}

// WithAccept overrides the Accept header sent by FetchURL.
func WithAccept(accept string) Option {
	return func(cfg *clientConfig) error {
		accept = strings.TrimSpace(accept)
		if accept == "" {
			return errors.New("accept header must not be empty")
		}
		cfg.accept = accept
		return nil
	}
}

// WithMaxBytes limits the decoded response body size.
func WithMaxBytes(maxBytes int64) Option {
	return func(cfg *clientConfig) error {
		if maxBytes <= 0 {
			return errors.New("max bytes must be greater than zero")
		}
		cfg.maxBytes = maxBytes
		return nil
	}
}

// WithTimeout sets the timeout on the default HTTP client created by New.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *clientConfig) error {
		if timeout <= 0 {
			return errors.New("timeout must be greater than zero")
		}
		cfg.timeout = timeout
		return nil
	}
}

// WithClientCertificate configures PEM client certificate files for mTLS.
func WithClientCertificate(certFile string, keyFile string) Option {
	return func(cfg *clientConfig) error {
		certFile = strings.TrimSpace(certFile)
		keyFile = strings.TrimSpace(keyFile)
		if certFile == "" && keyFile == "" {
			return nil
		}
		if certFile == "" || keyFile == "" {
			return errors.New("client certificate requires cert file and key file")
		}
		cfg.certFile = certFile
		cfg.keyFile = keyFile
		return nil
	}
}

// WithRootCA appends a PEM root CA bundle to the system trust store.
func WithRootCA(caFile string) Option {
	return func(cfg *clientConfig) error {
		cfg.caFile = strings.TrimSpace(caFile)
		return nil
	}
}

// WithInsecureSkipVerify disables server certificate verification.
// It is intended only for closed test systems.
func WithInsecureSkipVerify(enabled bool) Option {
	return func(cfg *clientConfig) error {
		cfg.insecureSkipVerify = enabled
		return nil
	}
}

// WithMaxRetries enables automatic retries in FetchURL for transient network
// errors and HTTP 429/5xx responses, using exponential backoff with jitter
// between attempts. The default, 0, disables retries entirely so existing
// callers see unchanged behavior unless they opt in.
func WithMaxRetries(maxRetries int) Option {
	return func(cfg *clientConfig) error {
		if maxRetries < 0 {
			return errors.New("max retries must not be negative")
		}
		cfg.maxRetries = maxRetries
		return nil
	}
}

// WithRetryBackoff sets the base delay used by the exponential backoff
// enabled through WithMaxRetries. The default is 200ms.
func WithRetryBackoff(base time.Duration) Option {
	return func(cfg *clientConfig) error {
		if base <= 0 {
			return errors.New("retry backoff must be greater than zero")
		}
		cfg.retryBackoff = base
		return nil
	}
}

// FetchSubscription tries one Mobilithek subscription endpoint, or all known
// candidates when endpoint is EndpointAuto.
func (c *Client) FetchSubscription(ctx context.Context, subscriptionID string, endpoint EndpointKind) (*Response, error) {
	return c.FetchSubscriptionWithHeaders(ctx, subscriptionID, endpoint, nil)
}

// FetchSubscriptionWithHeaders is like FetchSubscription, but sends additional
// HTTP headers such as If-None-Match or If-Modified-Since.
func (c *Client) FetchSubscriptionWithHeaders(ctx context.Context, subscriptionID string, endpoint EndpointKind, headers map[string]string) (*Response, error) {
	urls, err := CandidateURLs(c.baseURL, subscriptionID, endpoint)
	if err != nil {
		return nil, err
	}

	auto := normalizeEndpoint(endpoint) == EndpointAuto
	var lastErr error
	for _, candidate := range urls {
		response, err := c.FetchURL(ctx, candidate, headers)
		if err != nil {
			lastErr = err
			continue
		}
		if !auto || response.OK() || response.StatusCode == http.StatusNotModified {
			return response, nil
		}
		lastErr = &StatusError{
			URL:        response.URL,
			StatusCode: response.StatusCode,
			Body:       response.Body,
		}
	}

	if lastErr == nil {
		lastErr = errors.New("no Mobilithek endpoint returned a response")
	}
	return nil, lastErr
}

// FetchURL performs a GET request and returns response metadata plus the
// decoded body, respecting gzip encoding and the configured size limit. If
// WithMaxRetries was used to configure the client, transient network errors
// and HTTP 429/5xx responses are retried with exponential backoff and
// jitter; otherwise a single attempt is made.
//
// Any HTTP response, including a non-2xx one, is still returned with a nil
// error once attempts are exhausted or a non-retryable status is seen — the
// same "caller inspects StatusCode" contract FetchURL always had. Only a
// request that never produced an HTTP response (a transport-level failure
// on every attempt) results in a non-nil error.
func (c *Client) FetchURL(ctx context.Context, rawURL string, headers map[string]string) (*Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if attempt > 0 {
			if err := sleepWithContext(ctx, retryDelay(attempt-1, c.retryBackoff)); err != nil {
				return nil, err
			}
		}

		response, err := c.fetchURLOnce(ctx, rawURL, headers)
		if err != nil {
			lastErr = err
			continue
		}
		if attempt < c.maxRetries && isRetryableStatus(response.StatusCode) {
			continue
		}
		return response, nil
	}
	return nil, lastErr
}

func (c *Client) fetchURLOnce(ctx context.Context, rawURL string, headers map[string]string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", c.accept)
	req.Header.Set("Accept-Encoding", "gzip")
	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			req.Header.Set(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp, c.maxBytes)
	if err != nil {
		return nil, err
	}

	return &Response{
		URL:        rawURL,
		StatusCode: resp.StatusCode,
		Header:     resp.Header.Clone(),
		Body:       body,
	}, nil
}

func newHTTPClient(cfg clientConfig) (*http.Client, error) {
	tlsConfig := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		InsecureSkipVerify: cfg.insecureSkipVerify, //nolint:gosec // Explicit option for closed test systems.
	}

	if cfg.certFile != "" || cfg.keyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.certFile, cfg.keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.caFile != "" {
		caBytes, err := os.ReadFile(cfg.caFile)
		if err != nil {
			return nil, fmt.Errorf("read root CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if ok := pool.AppendCertsFromPEM(caBytes); !ok {
			return nil, errors.New("root CA file contains no valid PEM certificates")
		}
		tlsConfig.RootCAs = pool
	}

	// http.DefaultTransport is a mutable package-level variable; another
	// package in the same process (e.g. an HTTP auto-instrumentation
	// library) may have replaced it with a RoundTripper that is not a
	// *http.Transport, so this cannot assume the type assertion succeeds.
	transport := &http.Transport{}
	if defaultTransport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = defaultTransport.Clone()
	}
	transport.TLSClientConfig = tlsConfig

	return &http.Client{
		Transport: transport,
		Timeout:   cfg.timeout,
	}, nil
}

func readResponseBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	body, err := readLimited(resp.Body, maxBytes)
	if err != nil {
		return nil, err
	}

	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") || looksGzip(body) {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("decompress gzip response: %w", err)
		}
		defer zr.Close()
		return readLimited(zr, maxBytes)
	}

	return body, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	var buffer bytes.Buffer
	limited := io.LimitReader(reader, maxBytes+1)
	if _, err := buffer.ReadFrom(limited); err != nil {
		return nil, err
	}
	if int64(buffer.Len()) > maxBytes {
		return nil, fmt.Errorf("response exceeds max bytes (%d)", maxBytes)
	}
	return buffer.Bytes(), nil
}

func looksGzip(body []byte) bool {
	return len(body) >= 2 && body[0] == 0x1f && body[1] == 0x8b
}
