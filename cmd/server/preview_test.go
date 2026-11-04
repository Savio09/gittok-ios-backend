package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/config"
	"github.com/Savio09/gittok-ios-backend/internal/feed"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// fakeGitHubAPI serves one public repository and counts requests by endpoint.
type fakeGitHubAPI struct {
	mu     sync.Mutex
	counts map[string]int
	scopes []string // X-OAuth-Scopes values; nil omits the header
	srv    *httptest.Server
}

func newFakeGitHubAPI(t *testing.T, scopes []string) *fakeGitHubAPI {
	t.Helper()
	f := &fakeGitHubAPI{counts: map[string]int{}, scopes: scopes}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		endpoint := "other"
		switch {
		case r.URL.Path == "/rate_limit":
			endpoint = "rate_limit"
		case r.URL.Path == "/repos/o/r":
			endpoint = "metadata"
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/commits/"):
			endpoint = "ref"
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/git/trees/"):
			endpoint = "tree"
		case strings.HasPrefix(r.URL.Path, "/repos/o/r/contents/"):
			endpoint = "contents"
		}
		f.mu.Lock()
		f.counts[endpoint]++
		f.mu.Unlock()
		if f.scopes != nil {
			w.Header()["X-Oauth-Scopes"] = f.scopes
		}
		switch endpoint {
		case "rate_limit":
			_, _ = w.Write([]byte(`{}`))
		case "metadata":
			_, _ = w.Write([]byte(`{"id":123,"owner":{"login":"o"},"name":"r","full_name":"o/r","private":false,"default_branch":"main"}`))
		case "ref":
			_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
		case "tree":
			_, _ = w.Write([]byte(`{"tree":[{"path":"package.json","type":"blob","mode":"100644","size":40}]}`))
		case "contents":
			_, _ = w.Write([]byte(`{"type":"file","encoding":"base64","size":38,"content":"eyJuYW1lIjoieCIsImV4cG9ydHMiOiIuL2kuanMifQ=="}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeGitHubAPI) snapshot() map[string]int {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[string]int{}
	for k, v := range f.counts {
		out[k] = v
	}
	return out
}

func previewConfig(t *testing.T, token string) config.Config {
	return config.Config{GitHubToken: "feed", PublicToken: token, PreviewCacheTTL: 30 * time.Minute, PreviewCacheDir: t.TempDir()}
}

func get(router http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// Checkpoint C: HTTP request counts by endpoint for a cold preview and a warm
// repeat, through the real router and client.
func TestPreviewRequestEconomics(t *testing.T) {
	api := newFakeGitHubAPI(t, nil)
	svc, mode, err := previewService(context.Background(), previewConfig(t, ""), githubclient.WithBaseURL(api.srv.URL))
	if err != nil || !strings.Contains(mode, "anonymous") {
		t.Fatalf("anonymous preview service: %q %v", mode, err)
	}
	router := newRouter(feed.NewService(nil, nil, time.Minute), svc)
	if rec := get(router, "/v1/repos/o%2Fr/preview"); rec.Code != 200 {
		t.Fatalf("cold: %d %s", rec.Code, rec.Body.String())
	}
	cold := api.snapshot()
	// metadata, ref, tree, 1 selected file, identity recheck = selected + 4.
	if cold["metadata"] != 2 || cold["ref"] != 1 || cold["tree"] != 1 || cold["contents"] != 1 {
		t.Errorf("cold requests by endpoint: %v", cold)
	}
	if rec := get(router, "/v1/repos/o/r/preview"); rec.Code != 200 {
		t.Fatalf("warm: %d %s", rec.Code, rec.Body.String())
	}
	warm := api.snapshot()
	if warm["metadata"]-cold["metadata"] != 1 || warm["ref"] != cold["ref"] || warm["tree"] != cold["tree"] || warm["contents"] != cold["contents"] {
		t.Errorf("warm hit made more than one visibility lookup: cold %v, warm %v", cold, warm)
	}
	t.Logf("cold requests %v; warm adds metadata=%d, ref/tree/contents=0", cold, warm["metadata"]-cold["metadata"])
}

func TestPreviewResponsesForbidIntermediaryCaching(t *testing.T) {
	api := newFakeGitHubAPI(t, nil)
	svc, _, err := previewService(context.Background(), previewConfig(t, ""), githubclient.WithBaseURL(api.srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	router := newRouter(feed.NewService(nil, nil, time.Minute), svc)
	for path, want := range map[string]int{
		"/v1/repos/o%2Fr/preview":     200,
		"/v1/repos/o/r/preview":       200,
		"/v1/repos/not-valid/preview": 400,
		"/v1/repos/o/missing/preview": 404,
	} {
		rec := get(router, path)
		if rec.Code != want {
			t.Errorf("%s: status %d, want %d (%s)", path, rec.Code, want, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: Cache-Control %q, want no-store", path, rec.Header().Get("Cache-Control"))
		}
	}
}

// A configured public credential is verified at startup; an unsafe one stops
// startup rather than silently falling back to anonymous access.
func TestPublicCredentialStartup(t *testing.T) {
	token := "ghp_" + strings.Repeat("0", 36)
	for name, tc := range map[string]struct {
		token  string
		scopes []string
		mode   string
		fail   bool
	}{
		"anonymous":            {"", nil, "anonymous", false},
		"no-scope classic PAT": {token, []string{""}, "public-only token", false},
		"privileged scopes":    {token, []string{"repo"}, "", true},
		"no scope evidence":    {token, nil, "", true},
		"fine-grained token":   {"github_pat_" + strings.Repeat("0", 30), []string{""}, "", true},
	} {
		t.Run(name, func(t *testing.T) {
			api := newFakeGitHubAPI(t, tc.scopes)
			_, mode, err := previewService(context.Background(), previewConfig(t, tc.token), githubclient.WithBaseURL(api.srv.URL))
			if tc.fail {
				if err == nil {
					t.Fatalf("unsafe credential accepted (mode %q)", mode)
				}
				if strings.Contains(err.Error(), tc.token) {
					t.Fatalf("startup error leaks the credential: %v", err)
				}
				return
			}
			if err != nil || !strings.Contains(mode, tc.mode) {
				t.Fatalf("mode %q, err %v", mode, err)
			}
			if strings.Contains(mode, token) {
				t.Fatal("mode report leaks the credential")
			}
		})
	}
}
