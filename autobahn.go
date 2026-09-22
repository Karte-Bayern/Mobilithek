package mobilithek

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultAutobahnBaseURL is the public Autobahn API used by the German
// Autobahn-App. Unlike Mobilithek it needs no subscription or client
// certificate; it is a second, independent open-data source that this
// package can convert into the same Event/GeoJSON model.
const DefaultAutobahnBaseURL = "https://verkehr.autobahn.de/o/autobahn"

// AutobahnServiceKind selects one category of data from the Autobahn API.
type AutobahnServiceKind string

const (
	AutobahnRoadworks                   AutobahnServiceKind = "roadworks"
	AutobahnWarning                     AutobahnServiceKind = "warning"
	AutobahnClosure                     AutobahnServiceKind = "closure"
	AutobahnElectricChargingStation     AutobahnServiceKind = "electric_charging_station"
	AutobahnParkingLorryParkingFeatures AutobahnServiceKind = "parking_lorryparkingfeatures"
)

// AllAutobahnServiceKinds lists every service kind this package knows how to
// fetch and convert.
func AllAutobahnServiceKinds() []AutobahnServiceKind {
	return []AutobahnServiceKind{
		AutobahnRoadworks,
		AutobahnWarning,
		AutobahnClosure,
		AutobahnElectricChargingStation,
		AutobahnParkingLorryParkingFeatures,
	}
}

// AutobahnClient fetches data from the public Autobahn API.
type AutobahnClient struct {
	baseURL      string
	httpClient   *http.Client
	userAgent    string
	maxBytes     int64
	maxRetries   int
	retryBackoff time.Duration
}

// AutobahnOption configures an AutobahnClient created by NewAutobahnClient.
type AutobahnOption func(*autobahnConfig) error

type autobahnConfig struct {
	baseURL      string
	httpClient   *http.Client
	userAgent    string
	maxBytes     int64
	timeout      time.Duration
	maxRetries   int
	retryBackoff time.Duration
}

// NewAutobahnClient creates an Autobahn API client with conservative
// defaults, including a couple of automatic retries since the upstream API
// has no documented SLA.
func NewAutobahnClient(options ...AutobahnOption) (*AutobahnClient, error) {
	cfg := autobahnConfig{
		baseURL:      DefaultAutobahnBaseURL,
		userAgent:    defaultUserAgent,
		maxBytes:     defaultMaxBytes,
		timeout:      30 * time.Second,
		maxRetries:   2,
		retryBackoff: 250 * time.Millisecond,
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
		httpClient = &http.Client{Timeout: cfg.timeout}
	}

	return &AutobahnClient{
		baseURL:      strings.TrimRight(cfg.baseURL, "/"),
		httpClient:   httpClient,
		userAgent:    cfg.userAgent,
		maxBytes:     cfg.maxBytes,
		maxRetries:   cfg.maxRetries,
		retryBackoff: cfg.retryBackoff,
	}, nil
}

// WithAutobahnBaseURL overrides the Autobahn API base URL.
func WithAutobahnBaseURL(baseURL string) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
		if baseURL == "" {
			return errors.New("base URL must not be empty")
		}
		cfg.baseURL = baseURL
		return nil
	}
}

// WithAutobahnHTTPClient uses a caller-provided HTTP client.
func WithAutobahnHTTPClient(httpClient *http.Client) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		if httpClient == nil {
			return errors.New("HTTP client must not be nil")
		}
		cfg.httpClient = httpClient
		return nil
	}
}

// WithAutobahnUserAgent overrides the User-Agent header.
func WithAutobahnUserAgent(userAgent string) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		userAgent = strings.TrimSpace(userAgent)
		if userAgent == "" {
			return errors.New("user agent must not be empty")
		}
		cfg.userAgent = userAgent
		return nil
	}
}

// WithAutobahnMaxBytes limits the decoded response body size.
func WithAutobahnMaxBytes(maxBytes int64) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		if maxBytes <= 0 {
			return errors.New("max bytes must be greater than zero")
		}
		cfg.maxBytes = maxBytes
		return nil
	}
}

// WithAutobahnTimeout sets the timeout on the default HTTP client created by
// NewAutobahnClient.
func WithAutobahnTimeout(timeout time.Duration) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		if timeout <= 0 {
			return errors.New("timeout must be greater than zero")
		}
		cfg.timeout = timeout
		return nil
	}
}

// WithAutobahnMaxRetries sets how many times a failed request is retried.
// The default is 2; pass 0 to disable retries.
func WithAutobahnMaxRetries(maxRetries int) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		if maxRetries < 0 {
			return errors.New("max retries must not be negative")
		}
		cfg.maxRetries = maxRetries
		return nil
	}
}

// WithAutobahnRetryBackoff sets the base delay used by the exponential
// backoff between retries. The default is 250ms.
func WithAutobahnRetryBackoff(base time.Duration) AutobahnOption {
	return func(cfg *autobahnConfig) error {
		if base <= 0 {
			return errors.New("retry backoff must be greater than zero")
		}
		cfg.retryBackoff = base
		return nil
	}
}

// Roads returns the road identifiers (e.g. "A9") the Autobahn API currently
// exposes data for.
func (c *AutobahnClient) Roads(ctx context.Context) ([]string, error) {
	body, err := c.get(ctx, c.baseURL)
	if err != nil {
		return nil, err
	}

	var payload struct {
		Roads []string `json:"roads"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Autobahn roads list: %w", err)
	}
	return payload.Roads, nil
}

// FetchService fetches one service kind for one road and converts every
// result into the package's generic Event model, so it can be combined with
// Mobilithek-derived events through EventsToGeoJSON or MergeGeoJSON.
func (c *AutobahnClient) FetchService(ctx context.Context, roadID string, kind AutobahnServiceKind) ([]Event, error) {
	roadID = strings.TrimSpace(roadID)
	if roadID == "" {
		return nil, errors.New("road ID is required")
	}
	if strings.TrimSpace(string(kind)) == "" {
		return nil, errors.New("service kind is required")
	}

	endpoint := fmt.Sprintf("%s/%s/services/%s", c.baseURL, url.PathEscape(roadID), url.PathEscape(string(kind)))
	body, err := c.get(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode Autobahn %s response: %w", kind, err)
	}

	raw, ok := payload[string(kind)]
	if !ok {
		return []Event{}, nil
	}

	var items []autobahnItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("decode Autobahn %s items: %w", kind, err)
	}

	events := make([]Event, 0, len(items))
	for _, item := range items {
		events = append(events, autobahnItemToEvent(roadID, kind, item))
	}
	return events, nil
}

// FetchServices fetches multiple service kinds for one road and combines the
// results. It stops and returns an error on the first failed request.
func (c *AutobahnClient) FetchServices(ctx context.Context, roadID string, kinds []AutobahnServiceKind) ([]Event, error) {
	events := []Event{}
	for _, kind := range kinds {
		kindEvents, err := c.FetchService(ctx, roadID, kind)
		if err != nil {
			return nil, fmt.Errorf("road %s service %s: %w", roadID, kind, err)
		}
		events = append(events, kindEvents...)
	}
	return events, nil
}

func (c *AutobahnClient) get(ctx context.Context, rawURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			if err := sleepWithContext(ctx, retryDelay(attempt-1, c.retryBackoff)); err != nil {
				return nil, err
			}
		}

		body, statusCode, err := c.getOnce(ctx, rawURL)
		switch {
		case err != nil:
			lastErr = err
		case statusCode < 200 || statusCode >= 300:
			if attempt < c.maxRetries && isRetryableStatus(statusCode) {
				lastErr = &StatusError{URL: rawURL, StatusCode: statusCode, Body: body}
			} else {
				return nil, &StatusError{URL: rawURL, StatusCode: statusCode, Body: body}
			}
		default:
			return body, nil
		}

		if attempt >= c.maxRetries {
			return nil, lastErr
		}
	}
}

func (c *AutobahnClient) getOnce(ctx context.Context, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := readResponseBody(resp, c.maxBytes)
	if err != nil {
		return nil, 0, err
	}
	return body, resp.StatusCode, nil
}

// autobahnItem mirrors the fields observed across every Autobahn API service
// kind. Fields are intentionally string-typed where the upstream API itself
// is inconsistent (e.g. isBlocked is a JSON string, not a boolean). The
// "point" field is deliberately not modeled: it is a "lat,lon"-looking
// string, but the component order actually mirrors whatever order the
// "coordinate" object's keys happen to serialize in for that particular
// service (lat,long for roadworks; long,lat for electric_charging_station),
// which makes it unsafe to parse positionally. "coordinate" has explicit
// lat/long keys and is used instead.
type autobahnItem struct {
	Identifier          string            `json:"identifier"`
	IsBlocked           string            `json:"isBlocked"`
	Future              bool              `json:"future"`
	Coordinate          *autobahnLatLong  `json:"coordinate"`
	DisplayType         string            `json:"display_type"`
	Subtitle            string            `json:"subtitle"`
	Title               string            `json:"title"`
	StartTimestamp      string            `json:"startTimestamp"`
	Description         []string          `json:"description"`
	RouteRecommendation []string          `json:"routeRecommendation"`
	Geometry            *autobahnGeometry `json:"geometry"`
}

type autobahnGeometry struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// autobahnLatLong decodes the Autobahn API's "coordinate" object, whose lat
// and long values are numbers on some service kinds and JSON strings on
// others.
type autobahnLatLong struct {
	Lat  autobahnFloat `json:"lat"`
	Long autobahnFloat `json:"long"`
}

type autobahnFloat float64

func (f *autobahnFloat) UnmarshalJSON(data []byte) error {
	trimmed := strings.Trim(string(data), `"`)
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	value, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return fmt.Errorf("parse Autobahn coordinate value %q: %w", trimmed, err)
	}
	*f = autobahnFloat(value)
	return nil
}

func autobahnItemToEvent(roadID string, kind AutobahnServiceKind, item autobahnItem) Event {
	event := Event{
		Source: "autobahn-api",
		ID:     item.Identifier,
		Type:   item.DisplayType,
		Title:  strings.TrimSpace(item.Title),
		Road:   roadID,
		Start:  item.StartTimestamp,
		Status: autobahnStatus(item.Future),
		Extra:  map[string][]string{},
	}

	if description := strings.TrimSpace(strings.Join(nonEmptyLines(item.Description), " ")); description != "" {
		event.Description = truncate(description, 2000)
	}
	if item.Subtitle != "" {
		appendExtra(&event, "subtitle", strings.TrimSpace(item.Subtitle))
	}
	if item.IsBlocked != "" {
		appendExtra(&event, "blocked", item.IsBlocked)
	}
	appendExtra(&event, "service", string(kind))
	for _, recommendation := range item.RouteRecommendation {
		if strings.TrimSpace(recommendation) != "" {
			appendExtra(&event, "route_recommendation", recommendation)
		}
	}

	if coordinates := decodeAutobahnGeometry(item.Geometry); len(coordinates) > 0 {
		event.Coordinates = coordinates
	} else if item.Coordinate != nil {
		lat, lon := float64(item.Coordinate.Lat), float64(item.Coordinate.Long)
		if validLatLon(lat, lon) {
			event.Coordinates = []Coordinate{{Lat: lat, Lon: lon}}
		}
	}
	event.Coordinates = uniqueCoordinates(event.Coordinates, 250)

	if event.Title == "" {
		event.Title = deriveEventTitle(event)
	}
	if len(event.Extra) == 0 {
		event.Extra = nil
	}
	return event
}

func autobahnStatus(future bool) string {
	if future {
		return "future"
	}
	return "current"
}

func nonEmptyLines(lines []string) []string {
	output := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			output = append(output, line)
		}
	}
	return output
}

// decodeAutobahnGeometry converts a GeoJSON-shaped Point or LineString
// geometry (as returned by the Autobahn API, already in [lon, lat] order)
// into this package's Coordinate list.
func decodeAutobahnGeometry(geom *autobahnGeometry) []Coordinate {
	if geom == nil || len(geom.Coordinates) == 0 {
		return nil
	}

	if strings.EqualFold(geom.Type, "Point") {
		var pair []float64
		if err := json.Unmarshal(geom.Coordinates, &pair); err != nil || len(pair) < 2 {
			return nil
		}
		if !validLatLon(pair[1], pair[0]) {
			return nil
		}
		return []Coordinate{{Lat: pair[1], Lon: pair[0]}}
	}

	var pairs [][]float64
	if err := json.Unmarshal(geom.Coordinates, &pairs); err != nil {
		return nil
	}
	coordinates := make([]Coordinate, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 || !validLatLon(pair[1], pair[0]) {
			continue
		}
		coordinates = append(coordinates, Coordinate{Lat: pair[1], Lon: pair[0]})
	}
	return coordinates
}
