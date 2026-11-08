package feed

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type searchFake struct {
	mu     sync.Mutex
	calls  int
	pages  []int
	result githubclient.RepositorySearch
	err    error
}

func (f *searchFake) SearchRecentPublicRepositories(_ context.Context, _ time.Time, page int) (githubclient.RepositorySearch, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.pages = append(f.pages, page)
	return f.result, f.err
}
func testDiscovery(f *searchFake) *DiscoveryService {
	s := NewDiscoveryService(f, 10*time.Minute)
	s.now = func() time.Time { return time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC) }
	s.interval = 0
	return s
}
func discoveryRepo(n int) githubclient.Repo {
	return githubclient.Repo{ID: int64(n), Owner: "acme", Name: fmt.Sprintf("project-%d", n), FullName: fmt.Sprintf("acme/project-%d", n), Public: true}
}

func TestDiscoveryPaginatesPublicMetadataUsingRawProviderCount(t *testing.T) {
	f := &searchFake{result: githubclient.RepositorySearch{TotalCount: 70}}
	for i := 1; i <= 25; i++ {
		f.result.Repos = append(f.result.Repos, discoveryRepo(i))
	}
	f.result.Repos[0].Public = false
	f.result.Repos[1].Fork = true
	f.result.Repos[2].Archived = true
	s := testDiscovery(f)
	page, err := s.Feed(context.Background(), "")
	if err != nil || len(page.Items) != 22 || page.NextCursor == "" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	if page.Items[0].GitHubID != 4 || page.Items[0].Presentation != "" || len(page.Items[0].Sections) != 0 {
		t.Fatalf("card invented analysis: %+v", page.Items[0])
	}
	if _, err = s.Feed(context.Background(), page.NextCursor); err != nil {
		t.Fatal(err)
	}
	if len(f.pages) != 2 || f.pages[1] != 2 {
		t.Fatalf("requested pages=%v", f.pages)
	}
}

func TestDiscoveryCachesEmptyAndCoalescesConcurrentMisses(t *testing.T) {
	f := &searchFake{}
	s := testDiscovery(f)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, e := s.Feed(context.Background(), "")
			if e != nil || p.Items == nil || len(p.Items) != 0 {
				t.Errorf("empty=%+v err=%v", p, e)
			}
		}()
	}
	wg.Wait()
	if f.calls != 1 {
		t.Fatalf("%d search calls for one cache miss", f.calls)
	}
}

func TestDiscoveryRejectsIncompleteAndRetriesWithoutCaching(t *testing.T) {
	f := &searchFake{result: githubclient.RepositorySearch{Incomplete: true, Repos: []githubclient.Repo{discoveryRepo(1)}}}
	s := testDiscovery(f)
	for i := 0; i < 2; i++ {
		p, e := s.Feed(context.Background(), "")
		if e == nil || len(p.Items) != 0 {
			t.Fatalf("partial presented as complete: %+v %v", p, e)
		}
		// A short error cooldown may coalesce an outage burst; after it expires
		// incomplete search must be retried rather than cached as a valid page.
		next := s.now().Add(11 * time.Second)
		s.now = func() time.Time { return next }
	}
	if f.calls != 2 {
		t.Fatalf("cached failed page: calls=%d", f.calls)
	}
}

func TestDiscoveryCoalescesConcurrentUpstreamFailures(t *testing.T) {
	f := &searchFake{err: errors.New("provider unavailable")}
	s := testDiscovery(f)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Feed(context.Background(), "")
			if err == nil {
				t.Error("failure became successful feed")
			}
		}()
	}
	wg.Wait()
	if f.calls != 1 {
		t.Fatalf("outage burst made %d provider requests", f.calls)
	}
}

func TestDiscoveryRejectsUnboundedOrExpiredCursorsBeforeProvider(t *testing.T) {
	f := &searchFake{}
	s := testDiscovery(f)
	enc := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, c := range []string{"bad", enc(`{"version":1,"date":"2026-11-05","page":2}`), enc(`{"version":1,"date":"2026-11-08","page":2}`), enc(`{"version":1,"date":"2026-11-07","page":41}`), enc(`{"version":1,"date":"2026-11-07","page":2,"q":"private"}`), enc(`{"version":1,"date":"2026-11-07","page":2} {}`)} {
		r := httptest.NewRequest("GET", "/v1/feed?cursor="+c, nil)
		w := httptest.NewRecorder()
		s.Handler()(w, r)
		if w.Code != 400 {
			t.Errorf("cursor=%s status=%d", c, w.Code)
		}
	}
	if f.calls != 0 {
		t.Fatal("invalid cursor reached provider")
	}
}

func TestDiscoveryStopsAtSearchHorizonAndShortPage(t *testing.T) {
	f := &searchFake{result: githubclient.RepositorySearch{TotalCount: 2000}}
	for i := 1; i <= 25; i++ {
		f.result.Repos = append(f.result.Repos, discoveryRepo(i))
	}
	s := testDiscovery(f)
	cursor := base64.RawURLEncoding.EncodeToString([]byte(`{"version":1,"date":"2026-11-07","page":40}`))
	p, e := s.Feed(context.Background(), cursor)
	if e != nil || p.NextCursor != "" {
		t.Fatalf("horizon %+v %v", p, e)
	}
	f.result.Repos = f.result.Repos[:3]
	p, e = s.Feed(context.Background(), "")
	if e != nil || p.NextCursor != "" {
		t.Fatalf("short %+v %v", p, e)
	}
}
