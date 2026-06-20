package mobilithek

import (
	"fmt"
	"mime"
	"net/http"
	"strings"
)

type Response struct {
	URL        string
	StatusCode int
	Header     http.Header
	Body       []byte
}

func (r *Response) OK() bool {
	return r != nil && r.StatusCode >= 200 && r.StatusCode < 300
}

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
