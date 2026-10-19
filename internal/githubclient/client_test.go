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

// newTestClient returns a Client pointed at h, with a token set so we can also
// assert the Authorization header. Retries are off by default so single-response
// tests behave deterministically; retry tests use newRetryClient instead.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-token", WithBaseURL(srv.URL), WithUserAgent("gittok-test"), WithMaxRetries(0))
}

// newRetryClient is like newTestClient but allows maxRetries and never actually
// sleeps between attempts, so backoff behavior can be tested instantly.
func newRetryClient(t *testing.T, maxRetries int, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New("test-token", WithBaseURL(srv.URL), WithUserAgent("gittok-test"), WithMaxRetries(maxRetries))
	c.sleep = func(time.Duration) {}
	return c
}

func TestGetRepoMapsFields(t *testing.T) {
	const body = `{
		"name": "orbit",
		"full_name": "acme/orbit",
		"owner": { "login": "acme" },
		"description": "A productivity dashboard.",
		"language": "TypeScript",
		"stargazers_count": 18400,
		"license": { "spdx_id": "MIT", "name": "MIT License" },
		"html_url": "https://github.com/acme/orbit",
		"pushed_at": "2026-10-10T12:00:00Z",
		"archived": false,
		"fork": false
	}`

	var gotPath, gotAuth, gotAccept, gotUA string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	})

	repo, err := c.GetRepo(context.Background(), "acme", "orbit")
	if err != nil {
		t.Fatalf("GetRepo error = %v", err)
	}

	if gotPath != "/repos/acme/orbit" {
		t.Errorf("request path = %q", gotPath)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want Bearer test-token", gotAuth)
	}
	if gotAccept != "application/vnd.github+json" {
		t.Errorf("Accept = %q", gotAccept)
	}
	if gotUA != "gittok-test" {
		t.Errorf("User-Agent = %q", gotUA)
	}

	if repo.FullName != "acme/orbit" || repo.Owner != "acme" || repo.Name != "orbit" {
		t.Errorf("identity wrong: %+v", repo)
	}
	if repo.Language != "TypeScript" || repo.Stars != 18400 || repo.License != "MIT" {
		t.Errorf("fields wrong: %+v", repo)
	}
	if repo.PushedAt.IsZero() {
		t.Errorf("PushedAt not parsed")
	}
}

func TestGetRepoUnidentifiedLicenseIsEmpty(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"name":"x","full_name":"a/x","owner":{"login":"a"},"license":{"spdx_id":"NOASSERTION","name":"Other"}}`))
	})
	repo, err := c.GetRepo(context.Background(), "a", "x")
	if err != nil {
		t.Fatalf("GetRepo error = %v", err)
	}
	if repo.License != "" {
		t.Errorf("License = %q, want empty for NOASSERTION", repo.License)
	}
}

func TestGetRepoNotFound(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Not Found"}`))
	})
	_, err := c.GetRepo(context.Background(), "nope", "nope")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestGetRepoRateLimited(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	})
	_, err := c.GetRepo(context.Background(), "a", "b")
	if !errors.Is(err, ErrRateLimited) {
		t.Errorf("err = %v, want ErrRateLimited", err)
	}
}

func TestGetRepoAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"boom"}`))
	})
	_, err := c.GetRepo(context.Background(), "a", "b")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError || apiErr.Message != "boom" {
		t.Errorf("APIError = %+v", apiErr)
	}
}

// okRepoBody is a minimal valid repo response.
const okRepoBody = `{"name":"x","full_name":"a/x","owner":{"login":"a"}}`

func Test5xxRetriesThenSucceeds(t *testing.T) {
	var calls int32
	c := newRetryClient(t, 3, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.WriteHeader(http.StatusBadGateway) // fail the first two attempts
			return
		}
		_, _ = w.Write([]byte(okRepoBody))
	})
	repo, err := c.GetRepo(context.Background(), "a", "x")
	if err != nil {
		t.Fatalf("GetRepo error = %v, want success after retries", err)
	}
	if repo.FullName != "a/x" {
		t.Errorf("repo = %+v", repo)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("made %d attempts, want 3", got)
	}
}

func Test5xxExhaustsRetriesThenAPIError(t *testing.T) {
	var calls int32
	c := newRetryClient(t, 2, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"message":"down"}`))
	})
	_, err := c.GetRepo(context.Background(), "a", "x")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want *APIError after exhausting retries", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 { // 1 initial + 2 retries
		t.Errorf("made %d attempts, want 3", got)
	}
}

func TestSecondaryRateLimitRetried(t *testing.T) {
	var calls int32
	c := newRetryClient(t, 2, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusForbidden) // secondary limit: retriable
			return
		}
		_, _ = w.Write([]byte(okRepoBody))
	})
	if _, err := c.GetRepo(context.Background(), "a", "x"); err != nil {
		t.Fatalf("GetRepo error = %v, want success after honoring Retry-After", err)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("made %d attempts, want 2", got)
	}
}

func TestPrimaryRateLimitNotRetried(t *testing.T) {
	var calls int32
	c := newRetryClient(t, 3, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden) // primary limit: fail fast
		_, _ = w.Write([]byte(`{"message":"API rate limit exceeded"}`))
	})
	_, err := c.GetRepo(context.Background(), "a", "x")
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("made %d attempts, want 1 (no retry on primary limit)", got)
	}
}

func Test404NotRetried(t *testing.T) {
	var calls int32
	c := newRetryClient(t, 3, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.GetRepo(context.Background(), "a", "x")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("made %d attempts, want 1 (404 is terminal)", got)
	}
}

func TestBackoffDelay(t *testing.T) {
	base := 500 * time.Millisecond
	// Exponential from base when no Retry-After hint.
	if d := backoffDelay(1, 0, base); d != 500*time.Millisecond {
		t.Errorf("attempt 1 = %v, want 500ms", d)
	}
	if d := backoffDelay(2, 0, base); d != time.Second {
		t.Errorf("attempt 2 = %v, want 1s", d)
	}
	if d := backoffDelay(3, 0, base); d != 2*time.Second {
		t.Errorf("attempt 3 = %v, want 2s", d)
	}
	// A larger Retry-After hint wins.
	if d := backoffDelay(1, 5*time.Second, base); d != 5*time.Second {
		t.Errorf("with Retry-After = %v, want 5s", d)
	}
	// Capped at maxBackoff.
	if d := backoffDelay(20, 0, base); d != maxBackoff {
		t.Errorf("attempt 20 = %v, want cap %v", d, maxBackoff)
	}
}
