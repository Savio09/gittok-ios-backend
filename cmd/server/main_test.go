package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/feed"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/selection"
)

// testRouter builds the router with a feed service whose fetcher is never
// exercised by these tests (they only hit /healthz).
func testRouter() http.Handler {
	return newRouter(feed.NewService(nil, nil, time.Minute), analysis.NewPublicService())
}

func TestPreviewRouteUsesPublicPipelineAndFeedStaysMetadataOnly(t *testing.T) {
	var auth []string
	var analysisCalls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r" {
			auth = append(auth, r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"id":123,"owner":{"login":"o"},"name":"r","full_name":"o/r","description":"An arbitrary repo","private":false,"default_branch":"main"}`))
			return
		}
		analysisCalls++
		http.NotFound(w, r)
	}))
	defer srv.Close()
	client := githubclient.New("test-shared-token", githubclient.WithBaseURL(srv.URL))
	router := newRouter(feed.NewService(client, []selection.Slug{{Owner: "o", Name: "r"}}, time.Minute), analysis.NewPublicService(githubclient.WithBaseURL(srv.URL)))
	feedRec := httptest.NewRecorder()
	router.ServeHTTP(feedRec, httptest.NewRequest("GET", "/v1/feed", nil))
	if feedRec.Code != 200 || analysisCalls != 0 {
		t.Fatal("feed performed file analysis")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/repos/o%2Fr/preview", nil))
	if rec.Code != 200 {
		t.Fatalf("preview route status %d: %s", rec.Code, rec.Body.String())
	}
	var p apimodel.RepoPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "o/r" || len(p.Sections) == 0 || analysisCalls != 1 {
		t.Fatalf("preview path unused: %+v", p)
	}
	if len(auth) != 2 || auth[0] == "" || auth[1] != "" {
		t.Fatalf("incorrect credential boundary: %v", auth)
	}
}

func TestHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want ok", body["status"])
	}
}

func TestHealthzRejectsNonGET(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/healthz", nil)
	rec := httptest.NewRecorder()

	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
