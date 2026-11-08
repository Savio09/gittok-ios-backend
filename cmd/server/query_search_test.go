package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/feed"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func TestQuerySearchRouteDoesOnlyPublicMetadataSearch(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/search/repositories" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected metadata request: %s", r.URL.Path)
		}
		query := r.URL.Query().Get("q")
		if strings.Contains(query, "created:") || strings.Contains(query, "stars:") || !strings.Contains(query, "fork:true") {
			t.Errorf("restricted search: %s", query)
		}
		fmt.Fprint(w, `{"total_count":1,"incomplete_results":false,"items":[{"id":8,"owner":{"login":"outside"},"name":"old-fork","full_name":"outside/old-fork","private":false,"archived":true,"fork":true}]}`)
	}))
	defer upstream.Close()
	client, err := githubclient.NewPublic("", githubclient.WithBaseURL(upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	router := newRouter(feed.NewPublicDiscoveryService(client, time.Minute), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/v1/repos/search?q=old-fork", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"slug":"outside/old-fork"`) || calls != 1 {
		t.Fatalf("status=%d calls=%d body=%s", w.Code, calls, w.Body.String())
	}
}
