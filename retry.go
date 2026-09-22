package mobilithek

import (
	"context"
	"math/rand"
	"net/http"
	"time"
)

func isRetryableStatus(statusCode int) bool {
	return statusCode == http.StatusTooManyRequests || statusCode >= 500
}

// retryDelay returns an exponentially increasing delay with jitter for the
// given zero-based retry attempt (0 = the delay before the first retry).
func retryDelay(attempt int, base time.Duration) time.Duration {
	if base <= 0 {
		base = 200 * time.Millisecond
	}
	if attempt > 20 {
		attempt = 20 // caps the shift so it cannot overflow into a negative duration
	}
	backoff := base << attempt
	jitter := time.Duration(rand.Int63n(int64(base) + 1))
	return backoff + jitter
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
