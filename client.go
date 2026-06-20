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

type Client struct {
	baseURL    string
	httpClient *http.Client
	userAgent  string
	accept     string
	maxBytes   int64
}

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
}

func New(options ...Option) (*Client, error) {
	cfg := clientConfig{
		baseURL:   DefaultBaseURL,
		userAgent: defaultUserAgent,
		accept:    "application/xml,text/xml,application/json,text/json,*/*",
		maxBytes:  defaultMaxBytes,
		timeout:   60 * time.Second,
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
		baseURL:    strings.TrimRight(cfg.baseURL, "/"),
		httpClient: httpClient,
		userAgent:  cfg.userAgent,
		accept:     cfg.accept,
		maxBytes:   cfg.maxBytes,
	}, nil
}

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

func WithHTTPClient(httpClient *http.Client) Option {
	return func(cfg *clientConfig) error {
		if httpClient == nil {
			return errors.New("HTTP client must not be nil")
		}
		cfg.httpClient = httpClient
		return nil
	}
}

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

func WithMaxBytes(maxBytes int64) Option {
	return func(cfg *clientConfig) error {
		if maxBytes <= 0 {
			return errors.New("max bytes must be greater than zero")
		}
		cfg.maxBytes = maxBytes
		return nil
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(cfg *clientConfig) error {
		if timeout <= 0 {
			return errors.New("timeout must be greater than zero")
		}
		cfg.timeout = timeout
		return nil
	}
}

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

func WithRootCA(caFile string) Option {
	return func(cfg *clientConfig) error {
		cfg.caFile = strings.TrimSpace(caFile)
		return nil
	}
}

func WithInsecureSkipVerify(enabled bool) Option {
	return func(cfg *clientConfig) error {
		cfg.insecureSkipVerify = enabled
		return nil
	}
}

func (c *Client) FetchSubscription(ctx context.Context, subscriptionID string, endpoint EndpointKind) (*Response, error) {
	urls, err := CandidateURLs(c.baseURL, subscriptionID, endpoint)
	if err != nil {
		return nil, err
	}

	auto := normalizeEndpoint(endpoint) == EndpointAuto
	var lastErr error
	for _, candidate := range urls {
		response, err := c.FetchURL(ctx, candidate, nil)
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

func (c *Client) FetchURL(ctx context.Context, rawURL string, headers map[string]string) (*Response, error) {
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

	transport := http.DefaultTransport.(*http.Transport).Clone()
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
