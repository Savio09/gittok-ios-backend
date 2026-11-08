package githubclient

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestSearchRecentPublicRepositoriesMapsMetadataAndFixedQuery(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/search/repositories" || q.Get("q") != "is:public fork:false archived:false stars:>=20 created:2026-10-08..2026-11-07" || q.Get("sort") != "stars" || q.Get("order") != "desc" || q.Get("page") != "2" || q.Get("per_page") != "25" {
			t.Errorf("unexpected search: %s", r.URL)
		}
		fmt.Fprint(w, `{"total_count":65,"incomplete_results":false,"items":[{"id":123,"name":"orbit","full_name":"acme/orbit","owner":{"login":"acme"},"private":false,"visibility":"public","stargazers_count":55,"description":null,"language":"Go"}]}`)
	})
	result, err := c.SearchRecentPublicRepositories(context.Background(), time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC), 2)
	if err != nil || result.TotalCount != 65 || result.Incomplete || len(result.Repos) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	r := result.Repos[0]
	if !r.Public || r.ID != 123 || r.FullName != "acme/orbit" || r.Metadata["stargazers_count"].Value != "55" || !r.Metadata["description"].Null {
		t.Fatalf("lost metadata: %+v", r)
	}
}

func TestSearchRecentPublicRepositoriesRejectsPagesWithoutHTTP(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected HTTP") })
	for _, page := range []int{0, 41, -1} {
		if _, err := c.SearchRecentPublicRepositories(context.Background(), time.Now(), page); err == nil {
			t.Errorf("accepted page %d", page)
		}
	}
}

func TestSearchDoesNotMultiplyRequestsWithRetries(t *testing.T) {
	calls := 0
	c := newRetryClient(t, 3, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503) })
	if _, err := c.SearchRecentPublicRepositories(context.Background(), time.Now(), 1); err == nil {
		t.Fatal("expected upstream error")
	}
	if calls != 1 {
		t.Fatalf("made %d search requests", calls)
	}
}
