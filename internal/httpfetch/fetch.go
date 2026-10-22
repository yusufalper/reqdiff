package httpfetch

import (
	"context"
	"errors"
	"github.com/yusufalper/reqdiff/internal/auth"
	"io"
	"net"
	"net/http"
	"time"
)

type Snapshot struct {
	Status   int
	Headers  http.Header
	Body     []byte
	Duration time.Duration
}
type Error struct{ Code string }

func (e *Error) Error() string {
	switch e.Code {
	case "cancelled":
		return "request cancelled"
	case "timeout":
		return "request timed out"
	case "body_too_large":
		return "response exceeds body limit"
	default:
		return "HTTP request failed"
	}
}
func NewClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		Transport:     http.DefaultTransport.(*http.Transport).Clone(),
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
func Fetch(ctx context.Context, client *http.Client, endpoint auth.ResolvedEndpoint, maxBytes int64) (Snapshot, error) {
	var snapshot Snapshot
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.URL, nil)
	if err != nil {
		return snapshot, &Error{Code: "network"}
	}
	req.Header = endpoint.Headers.Clone()
	start := time.Now()
	response, err := client.Do(req)
	if err != nil {
		snapshot.Duration = time.Since(start)
		return snapshot, classify(ctx, err)
	}
	snapshot.Status, snapshot.Headers = response.StatusCode, response.Header.Clone()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	closeErr := response.Body.Close()
	snapshot.Duration = time.Since(start)
	if err != nil {
		return snapshot, classify(ctx, err)
	}
	if closeErr != nil {
		return snapshot, classify(ctx, closeErr)
	}
	if int64(len(body)) > maxBytes {
		return snapshot, &Error{Code: "body_too_large"}
	}
	snapshot.Body = body
	return snapshot, nil
}
func classify(ctx context.Context, err error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return &Error{Code: "cancelled"}
	}
	var ne net.Error
	if errors.Is(err, context.DeadlineExceeded) || errors.As(err, &ne) && ne.Timeout() {
		return &Error{Code: "timeout"}
	}
	return &Error{Code: "network"}
}
