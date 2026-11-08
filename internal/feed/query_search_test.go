package feed

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func TestQuerySearchFindsPublicForksAndArchivedProjectsOutsideDiscover(t *testing.T) {
	f := &searchFake{result: githubclient.RepositorySearch{TotalCount: 30}}
	for i := 1; i <= 25; i++ {
		f.result.Repos = append(f.result.Repos, discoveryRepo(i))
	}
	f.result.Repos[0].Fork = true
	f.result.Repos[1].Archived = true
	f.result.Repos[2].Public = false
	s := testDiscovery(f)
	page, err := s.Search(context.Background(), "older project", "")
	if err != nil || len(page.Items) != 24 || page.NextCursor == "" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if _, err = s.Search(context.Background(), "different", page.NextCursor); err == nil {
		t.Fatal("accepted another query's cursor")
	}
	if f.calls != 1 {
		t.Fatalf("unexpected calls=%d", f.calls)
	}
	if _, err = s.Search(context.Background(), "older project", page.NextCursor); err != nil {
		t.Fatal(err)
	}
	if f.calls != 2 || f.pages[1] != 2 {
		t.Fatalf("pages=%v", f.pages)
	}
}

func TestQuerySearchCachesEmptyResultsAndBoundsCache(t *testing.T) {
	f := &searchFake{}
	s := testDiscovery(f)
	for i := 0; i < 2; i++ {
		p, err := s.Search(context.Background(), "nothing", "")
		if err != nil || p.Items == nil {
			t.Fatalf("page=%+v err=%v", p, err)
		}
	}
	if f.calls != 1 {
		t.Fatalf("calls=%d", f.calls)
	}
	for i := 1; i < 45; i++ {
		if _, err := s.Search(context.Background(), strings.Repeat("x", i), ""); err != nil {
			t.Fatal(err)
		}
	}
	if len(s.queryCache) > 32 {
		t.Fatalf("unbounded cache=%d", len(s.queryCache))
	}
}

func TestQuerySearchReturnsInputAndUpstreamFailuresAsDistinctHTTPStates(t *testing.T) {
	for _, tt := range []struct {
		name, query string
		err         error
		status      int
	}{
		{"blank", " ", nil, 400},
		{"syntax", "hello", &githubclient.APIError{StatusCode: 422, Message: "Validation Failed"}, 400},
		{"spam protection", "hello", &githubclient.APIError{StatusCode: 422, Message: "You have triggered an abuse detection mechanism."}, 503},
		{"quota", "hello", githubclient.ErrRateLimited, 429},
		{"failure", "hello", errors.New("secret provider detail"), 503},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := &searchFake{err: tt.err}
			s := testDiscovery(f)
			r := httptest.NewRequest("GET", "/v1/repos/search", nil)
			q := r.URL.Query()
			q.Set("q", tt.query)
			r.URL.RawQuery = q.Encode()
			w := httptest.NewRecorder()
			s.SearchHandler()(w, r)
			if w.Code != tt.status || strings.Contains(w.Body.String(), "provider detail") {
				t.Fatalf("%d %s", w.Code, w.Body.String())
			}
			if tt.status == 400 && tt.name == "blank" && f.calls != 0 {
				t.Fatal("blank query reached upstream")
			}
		})
	}
}

func TestQuerySearchCancelledWaitDoesNotSpendAnUpstreamRequest(t *testing.T) {
	f := &searchFake{}
	s := testDiscovery(f)
	s.gate <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.Search(ctx, "hello", ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	<-s.gate
	if f.calls != 0 {
		t.Fatal("cancelled search reached GitHub")
	}
}
