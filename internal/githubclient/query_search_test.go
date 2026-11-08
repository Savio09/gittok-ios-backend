package githubclient

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestPublicQuerySearchHasNoDiscoverRestrictions(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if r.URL.Path != "/search/repositories" || q.Get("q") != "whiteboard is:public fork:true" || q.Get("sort") != "" || q.Get("page") != "2" || q.Get("per_page") != "25" {
			t.Errorf("unexpected search: %s", r.URL)
		}
		fmt.Fprint(w, `{"total_count":26,"incomplete_results":false,"items":[{"id":123,"name":"old-fork","full_name":"someone/old-fork","owner":{"login":"someone"},"private":false,"visibility":"public","fork":true,"archived":true}]}`)
	})
	result, err := c.SearchPublicRepositories(context.Background(), " whiteboard ", 2)
	if err != nil || len(result.Repos) != 1 || !result.Repos[0].Public || !result.Repos[0].Fork || !result.Repos[0].Archived {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestPublicQuerySearchRejectsInvalidInputWithoutHTTP(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("unexpected HTTP") })
	for _, q := range []string{"", "  ", "hello\nworld", strings.Repeat("x", 257), string([]byte{0xff})} {
		if _, err := c.SearchPublicRepositories(context.Background(), q, 1); err == nil {
			t.Errorf("accepted invalid query %q", q)
		}
	}
	for _, p := range []int{0, 41} {
		if _, err := c.SearchPublicRepositories(context.Background(), "valid", p); err == nil {
			t.Errorf("accepted page %d", p)
		}
	}
}

func TestPublicQuerySearchNormalizesExactRepository(t *testing.T) {
	for _, query := range []string{"owner/project", "https://github.com/owner/project", "https://github.com/owner/project#readme", "https://github.com/owner/project?tab=readme-ov-file"} {
		t.Run(query, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("q") != "repo:owner/project is:public fork:true" {
					t.Errorf("query=%s", r.URL.Query().Get("q"))
				}
				fmt.Fprint(w, `{"total_count":0,"incomplete_results":false,"items":[]}`)
			})
			if _, err := c.SearchPublicRepositories(context.Background(), query, 1); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPublicQuerySearchNeverRetriesUpstreamFailures(t *testing.T) {
	calls := 0
	c := newRetryClient(t, 3, func(w http.ResponseWriter, r *http.Request) { calls++; w.WriteHeader(503) })
	if _, err := c.SearchPublicRepositories(context.Background(), "whiteboard", 1); err == nil {
		t.Fatal("expected upstream failure")
	}
	if calls != 1 {
		t.Fatalf("made %d calls", calls)
	}
}

func TestPublicQuerySearchRejectsMalformedProviderEnvelopes(t *testing.T) {
	for _, body := range []string{`null`, `{}`, `{"message":"failure"}`, `{"total_count":0,"incomplete_results":false,"items":null}`} {
		t.Run(body, func(t *testing.T) {
			c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
			if _, err := c.SearchPublicRepositories(context.Background(), "hello", 1); err == nil {
				t.Fatal("malformed provider response became a successful empty result")
			}
		})
	}
}

func TestPublicQuerySearchDoesNotTreatQuotedWordsAsQualifiers(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if q := r.URL.Query().Get("q"); q != `"a fork:false b" is:public fork:true` {
			t.Errorf("query=%s", q)
		}
		fmt.Fprint(w, `{"total_count":0,"incomplete_results":false,"items":[]}`)
	})
	if _, err := c.SearchPublicRepositories(context.Background(), `"a fork:false b"`, 1); err != nil {
		t.Fatal(err)
	}
}

func TestPublicQuerySearchCapsRedirectAttempts(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Redirect(w, r, "/redirected-search", http.StatusFound)
	})
	if _, err := c.SearchPublicRepositories(context.Background(), "hello", 1); err == nil {
		t.Fatal("accepted unbounded redirect")
	}
	if calls != 1 {
		t.Fatalf("made %d physical requests", calls)
	}
}
