package githubclient

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
)

var (
	ErrCallBudget       = errors.New("githubclient: API call budget exhausted")
	ErrResponseTooLarge = errors.New("githubclient: response byte limit exceeded")
)

// DefaultMaxResponseBytes bounds decoded HTTP response bytes, including when
// Content-Length is absent or false. Tree and content retention have lower caps.
const DefaultMaxResponseBytes = 16 << 20

// RequestBudget is shared only by one analysis, never by unrelated requests.
// It counts actual RoundTrips, so redirects and retries cannot bypass the cap.
// Context can attach the same budget to independently cancelled shared work.
type RequestBudget struct {
	mu            sync.Mutex
	remaining     int
	responseBytes int
}

// NewRequestBudget requires positive limits; invalid values select safe defaults.
func NewRequestBudget(maxCalls, maxResponseBytes int) *RequestBudget {
	if maxCalls <= 0 || maxCalls > 128 {
		maxCalls = 80
	}
	if maxResponseBytes <= 0 || maxResponseBytes > 32<<20 {
		maxResponseBytes = DefaultMaxResponseBytes
	}
	return &RequestBudget{remaining: maxCalls, responseBytes: maxResponseBytes}
}

type budgetKey struct{}

func (b *RequestBudget) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, budgetKey{}, b)
}

func requestBudget(ctx context.Context) *RequestBudget {
	b, _ := ctx.Value(budgetKey{}).(*RequestBudget)
	return b
}

func (b *RequestBudget) check(take bool) error {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.remaining == 0 {
		return ErrCallBudget
	}
	if take {
		b.remaining--
	}
	return nil
}

type budgetTransport struct{ base http.RoundTripper }

// RoundTrip is the physical request boundary (https://pkg.go.dev/net/http#RoundTripper).
func (t budgetTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := r.Context().Err(); err != nil {
		return nil, err
	}
	if err := requestBudget(r.Context()).check(true); err != nil {
		return nil, err
	}
	return t.base.RoundTrip(r)
}

// capResponses reads at most maxBytes+1; the extra byte detects an oversized
// body without trusting Content-Length. It also checks cancellation between
// reads. The HTTP transport handles cancellation during a blocked network read.
func capResponse(ctx context.Context, body io.ReadCloser) io.ReadCloser {
	limit := DefaultMaxResponseBytes
	if b := requestBudget(ctx); b != nil {
		limit = b.responseBytes
	}
	return &responseReader{ReadCloser: body, ctx: ctx, remaining: int64(limit)}
}

type responseReader struct {
	io.ReadCloser
	ctx       context.Context
	remaining int64
}

func (r *responseReader) Read(p []byte) (int, error) {
	if r.remaining < 0 {
		return 0, ErrResponseTooLarge
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > r.remaining+1 {
		p = p[:r.remaining+1]
	}
	n, err := r.ReadCloser.Read(p)
	allowed := r.remaining
	r.remaining -= int64(n)
	if r.remaining < 0 {
		// Never give json.Decoder the sentinel byte: it can accept a complete
		// JSON value from bytes returned alongside an error and ignore that error.
		return int(allowed), ErrResponseTooLarge
	}
	return n, err
}
