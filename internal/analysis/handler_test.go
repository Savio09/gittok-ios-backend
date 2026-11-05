package analysis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func previewRouter(s *Service) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /v1/repos/{id}/preview", s.Handler())
	return mux
}

func TestPublicPreviewUsesAnonymousPinnedAcquisition(t *testing.T) {
	for _, privateNow := range []bool{false, true} {
		t.Run(map[bool]string{false: "public", true: "became-private"}[privateNow], func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "" {
					t.Error("public endpoint used privileged credentials")
				}
				switch r.URL.Path {
				case "/repos/o/r":
					_, _ = w.Write([]byte(`{"id":123,"owner":{"login":"o"},"name":"r","full_name":"o/r","private":false,"description":"Public purpose","default_branch":"moving","language":"Ruby"}`))
				case "/repos/o/r/commits/moving":
					if privateNow {
						http.Error(w, `{"message":"became private"}`, 404)
						return
					}
					_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
				case "/repos/o/r/git/trees/0123456789abcdef0123456789abcdef01234567":
					_, _ = w.Write([]byte(`{"tree":[{"path":"README.md","type":"blob","mode":"100644"}]}`))
				case "/repos/o/r/contents/README.md":
					if r.URL.Query().Get("ref") != "0123456789abcdef0123456789abcdef01234567" {
						t.Error("mutable contents ref")
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"type": "file", "encoding": "base64", "content": base64.StdEncoding.EncodeToString([]byte("# A\n\nAn unfamiliar ecosystem.\n"))})
				default:
					t.Errorf("unexpected upstream path %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			s := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
			rec := httptest.NewRecorder()
			previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil))
			if rec.Code != 200 {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			var p apimodel.RepoPreview
			if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
				t.Fatal(err)
			}
			if p.ID != "o/r" || p.Kind != apimodel.KindOther {
				t.Fatalf("bad preview %+v", p)
			}
			if !strings.Contains(rec.Body.String(), "Public purpose") {
				t.Fatal("lost metadata")
			}
			if privateNow {
				if calls != 2 || strings.Contains(rec.Body.String(), `"kind":"file"`) {
					t.Fatalf("leaked after visibility change: %s", rec.Body.String())
				}
			} else if !strings.Contains(rec.Body.String(), "An unfamiliar ecosystem.") {
				t.Fatal("lost sourced documentation")
			}
		})
	}
}

func TestPublicPreviewRejectsPrivateAndUnknownVisibility(t *testing.T) {
	for _, visibility := range []string{`"private":true,`, ``, `"private":null,`, `"private":false,"visibility":"internal",`} {
		t.Run(visibility, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				_, _ = w.Write([]byte(`{` + visibility + `"name":"r","owner":{"login":"o"},"description":"SECRET","default_branch":"main"}`))
			}))
			defer srv.Close()
			s := NewPublicService(githubclient.WithBaseURL(srv.URL))
			rec := httptest.NewRecorder()
			previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil))
			if rec.Code != 404 || strings.Contains(rec.Body.String(), "SECRET") || calls != 1 {
				t.Fatalf("visibility leak: %d %s calls=%d", rec.Code, rec.Body.String(), calls)
			}
		})
	}
}

func TestPublicPreviewErrorsAreSanitized(t *testing.T) {
	for _, tc := range []struct{ upstream, want int }{{404, 404}, {401, 503}, {403, 503}, {429, 503}, {500, 502}} {
		t.Run(http.StatusText(tc.upstream), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, `{"message":"sensitive provider details"}`, tc.upstream)
			}))
			defer srv.Close()
			s := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
			rec := httptest.NewRecorder()
			previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil))
			if rec.Code != tc.want || strings.Contains(rec.Body.String(), "sensitive") {
				t.Fatalf("status/body %d %s", rec.Code, rec.Body.String())
			}
			var body map[string]string
			if json.Unmarshal(rec.Body.Bytes(), &body) != nil || body["error"] == "" {
				t.Fatal("changed error shape")
			}
		})
	}
}

func TestPublicPreviewInvalidIDsDoNotFetch(t *testing.T) {
	s := NewPublicService(githubclient.WithHTTPClient(&http.Client{Transport: denyTransport{t: t}}))
	for _, id := range []string{"o%2Fr%2Fx", "bare", "o%252Fr", "o%2F..", "o%2Fbad%3Fname"} {
		rec := httptest.NewRecorder()
		previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/"+id+"/preview", nil))
		if rec.Code != 400 {
			t.Fatalf("%s: status %d", id, rec.Code)
		}
	}
}

type denyTransport struct{ t *testing.T }

func (d denyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	d.t.Fatal("unexpected acquisition")
	return nil, context.Canceled
}

func TestPublicPreviewDeadlineAndCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	s := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
	s.timeout = 5 * time.Millisecond
	rec := httptest.NewRecorder()
	previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil))
	if rec.Code != 504 {
		t.Fatalf("deadline status %d", rec.Code)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rec = httptest.NewRecorder()
	previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil).WithContext(ctx))
	if rec.Body.Len() != 0 {
		t.Fatal("wrote success or error to canceled request")
	}
}

func TestTrustedTokenServiceCannotBeExposedAsPublicHandler(t *testing.T) {
	s := NewService(&fakeGitHub{})
	rec := httptest.NewRecorder()
	previewRouter(s).ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil))
	if rec.Code != 503 {
		t.Fatalf("trusted client exposed: %d", rec.Code)
	}
}
