package githubclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Slice 8c: a context cancelled during retry backoff must interrupt the wait
// promptly, not block for the full (up to 30s) backoff. The server always
// returns 500 so the client enters backoff; the injected sleep blocks until the
// test releases it, standing in for a long backoff, and the test cancels the
// context mid-wait.
func TestBackoffIsInterruptedByCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"boom"}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	release := make(chan struct{})
	defer close(release)

	c := New("t", WithBaseURL(srv.URL), WithUserAgent("gittok-test"), WithMaxRetries(3))
	c.sleep = func(time.Duration) { <-release } // a backoff that would block indefinitely

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := c.GetRepo(ctx, "owner", "repo")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Errorf("GetRepo took %s; backoff was not interrupted by cancellation", elapsed)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
