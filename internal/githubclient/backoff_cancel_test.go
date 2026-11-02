package githubclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Cancellation must end a fetch promptly wherever it lands: while a request is
// in flight on the only or the final path, or while waiting out a retry backoff.
// Every case synchronises on an event (request received, backoff entered) and
// then cancels; none relies on an arbitrary sleep for ordering.

// blockingServer signals each request it receives on arrived, then blocks the
// paths in block until the request's context ends; other paths get a small file.
func blockingServer(t *testing.T, block map[string]bool, arrived chan<- string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		arrived <- p
		if block[p] {
			<-r.Context().Done()
			return
		}
		_, _ = w.Write([]byte(`{"type":"file","encoding":"base64","size":3,"content":"YWJj"}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestCancellationDuringOnlyContentsRequest(t *testing.T) {
	arrived := make(chan string, 4)
	srv := blockingServer(t, map[string]bool{"/repos/o/r/contents/main.go": true}, arrived)
	c := New("", WithBaseURL(srv.URL), WithMaxRetries(0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { <-arrived; cancel() }()

	got, err := c.GetFileContents(ctx, "o", "r", "main", []string{"main.go"}, DefaultContentLimits())
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v (skipped %v), want context.Canceled", err, got.Skipped)
	}
	if len(got.Skipped) != 0 {
		t.Errorf("a cancelled fetch must not be recorded as a skipped file: %v", got.Skipped)
	}
}

func TestCancellationDuringFinalContentsRequest(t *testing.T) {
	arrived := make(chan string, 4)
	srv := blockingServer(t, map[string]bool{"/repos/o/r/contents/b.go": true}, arrived)
	c := New("", WithBaseURL(srv.URL), WithMaxRetries(0))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for p := range arrived {
			if p == "/repos/o/r/contents/b.go" {
				cancel()
				return
			}
		}
	}()

	got, err := c.GetFileContents(ctx, "o", "r", "main", []string{"a.go", "b.go"}, DefaultContentLimits())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "a.go" {
		t.Errorf("files read before cancellation should be returned: %+v", got.Files)
	}
}

// A deadline that passes during the final request is reported as such. The
// context is controlled by the test, so expiry happens exactly when the request
// has arrived — no wall-clock timing is involved.
func TestDeadlineDuringFinalContentsRequest(t *testing.T) {
	arrived := make(chan string, 4)
	srv := blockingServer(t, map[string]bool{"/repos/o/r/contents/main.go": true}, arrived)
	c := New("", WithBaseURL(srv.URL), WithMaxRetries(0))

	ctx := newExpiringContext()
	go func() { <-arrived; ctx.expire() }()

	if _, err := c.GetFileContents(ctx, "o", "r", "main", []string{"main.go"}, DefaultContentLimits()); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want context.DeadlineExceeded", err)
	}
}

// A context cancelled while the client waits out a retry backoff stops the
// fetch promptly. The injected sleep signals when the backoff starts and then
// blocks until the test releases it, standing in for a long backoff.
func TestCancellationDuringRetryBackoff(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	inBackoff, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	c := New("t", WithBaseURL(srv.URL), WithUserAgent("gittok-test"), WithMaxRetries(3))
	c.sleep = func(time.Duration) {
		inBackoff <- struct{}{}
		<-release
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-inBackoff; cancel() }()

	if _, err := c.GetFileContents(ctx, "o", "r", "main", []string{"main.go"}, DefaultContentLimits()); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("requests = %d, want 1: no retry may start after cancellation", n)
	}
}

// expiringContext is a context whose deadline the test triggers explicitly.
type expiringContext struct {
	context.Context
	done chan struct{}
}

func newExpiringContext() *expiringContext {
	return &expiringContext{Context: context.Background(), done: make(chan struct{})}
}

func (c *expiringContext) expire()                     { close(c.done) }
func (c *expiringContext) Done() <-chan struct{}       { return c.done }
func (c *expiringContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *expiringContext) Err() error {
	select {
	case <-c.done:
		return context.DeadlineExceeded
	default:
		return nil
	}
}
