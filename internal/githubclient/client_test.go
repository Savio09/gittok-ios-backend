package githubclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient returns a Client pointed at h, with a token set so we can also
// assert the Authorization header.
func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return New("test-token", WithBaseURL(srv.URL), WithUserAgent("gittok-test"))
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
