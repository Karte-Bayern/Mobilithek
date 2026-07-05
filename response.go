package mobilithek

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
	"time"
)

// Response contains the fetched URL, HTTP metadata, and decoded body.
type Response struct {
	URL        string
	StatusCode int
	Header     http.Header
	Body       []byte
}

// OK reports whether the response status code is in the 2xx range.
func (r *Response) OK() bool {
	return r != nil && r.StatusCode >= 200 && r.StatusCode < 300
}

// ContentType returns the parsed media type without parameters.
func (r *Response) ContentType() string {
	if r == nil {
		return ""
	}
	contentType := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return strings.TrimSpace(contentType)
	}
	return mediaType
}

// ETag returns the response ETag header.
func (r *Response) ETag() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get("ETag"))
}

// LastModified returns the Last-Modified header as sent by the server.
func (r *Response) LastModified() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.Header.Get("Last-Modified"))
}

// LastModifiedTime parses Last-Modified using HTTP-date rules.
func (r *Response) LastModifiedTime() (time.Time, bool) {
	lastModified := r.LastModified()
	if lastModified == "" {
		return time.Time{}, false
	}
	parsed, err := http.ParseTime(lastModified)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// StatusError reports a non-successful HTTP status and a short body excerpt.
type StatusError struct {
	URL        string
	StatusCode int
	Body       []byte
}

func (e *StatusError) Error() string {
	body := strings.TrimSpace(string(e.Body))
	if len(body) > 240 {
		body = body[:240] + "..."
	}
	if body == "" {
		return fmt.Sprintf("%s returned HTTP %d", e.URL, e.StatusCode)
	}
	return fmt.Sprintf("%s returned HTTP %d: %s", e.URL, e.StatusCode, body)
}
