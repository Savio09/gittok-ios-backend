package feed

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/selection"
)

// fakeFetcher returns canned repos (or errors) by name, and counts calls.
type fakeFetcher struct {
	repos map[string]githubclient.Repo
	errs  map[string]error
	calls int32
}

func (f *fakeFetcher) GetRepo(_ context.Context, owner, name string) (githubclient.Repo, error) {
	atomic.AddInt32(&f.calls, 1)
	if err := f.errs[name]; err != nil {
		return githubclient.Repo{}, err
	}
	r, ok := f.repos[name]
	if !ok {
		return githubclient.Repo{}, githubclient.ErrNotFound
	}
	return r, nil
}

func repo(owner, name, lang string) githubclient.Repo {
	return githubclient.Repo{
		Owner: owner, Name: name, FullName: owner + "/" + name,
		Language: lang, Stars: 1000, HTMLURL: "https://github.com/" + owner + "/" + name,
		PushedAt: time.Now().Add(-48 * time.Hour),
	}
}

func threeSlugs() []selection.Slug {
	return []selection.Slug{{Owner: "a", Name: "one"}, {Owner: "b", Name: "two"}, {Owner: "c", Name: "three"}}
}

func TestFeedPreservesOrder(t *testing.T) {
	f := &fakeFetcher{repos: map[string]githubclient.Repo{
		"one": repo("a", "one", "Go"), "two": repo("b", "two", "Rust"), "three": repo("c", "three", "TypeScript"),
	}}
	svc := NewService(f, threeSlugs(), time.Minute)

	feed, err := svc.Feed(context.Background())
	if err != nil {
		t.Fatalf("Feed error = %v", err)
	}
	want := []string{"a/one", "b/two", "c/three"}
	if len(feed.Items) != len(want) {
		t.Fatalf("got %d items, want %d", len(feed.Items), len(want))
	}
	for i, slug := range want {
		if feed.Items[i].Slug != slug {
			t.Errorf("item %d slug = %q, want %q", i, feed.Items[i].Slug, slug)
		}
	}
}

func TestFeedToleratesPartialFailure(t *testing.T) {
	f := &fakeFetcher{
		repos: map[string]githubclient.Repo{"one": repo("a", "one", "Go"), "three": repo("c", "three", "Rust")},
		errs:  map[string]error{"two": githubclient.ErrNotFound},
	}
	svc := NewService(f, threeSlugs(), time.Minute)

	feed, err := svc.Feed(context.Background())
	if err != nil {
		t.Fatalf("Feed error = %v (should tolerate one failure)", err)
	}
	if len(feed.Items) != 2 {
		t.Fatalf("got %d items, want 2", len(feed.Items))
	}
	if feed.Items[0].Slug != "a/one" || feed.Items[1].Slug != "c/three" {
		t.Errorf("surviving items in wrong order: %q, %q", feed.Items[0].Slug, feed.Items[1].Slug)
	}
}

func TestFeedAllFailReturnsError(t *testing.T) {
	f := &fakeFetcher{errs: map[string]error{
		"one": githubclient.ErrRateLimited, "two": githubclient.ErrNotFound, "three": githubclient.ErrRateLimited,
	}}
	svc := NewService(f, threeSlugs(), time.Minute)

	_, err := svc.Feed(context.Background())
	if err == nil {
		t.Fatal("Feed() = nil error, want error when every repo fails")
	}
}

func TestFeedCaches(t *testing.T) {
	f := &fakeFetcher{repos: map[string]githubclient.Repo{
		"one": repo("a", "one", "Go"), "two": repo("b", "two", "Rust"), "three": repo("c", "three", "TypeScript"),
	}}
	svc := NewService(f, threeSlugs(), time.Minute)

	if _, err := svc.Feed(context.Background()); err != nil {
		t.Fatalf("first Feed: %v", err)
	}
	firstCalls := atomic.LoadInt32(&f.calls)
	if _, err := svc.Feed(context.Background()); err != nil {
		t.Fatalf("second Feed: %v", err)
	}
	if got := atomic.LoadInt32(&f.calls); got != firstCalls {
		t.Errorf("second Feed made %d extra fetches, want 0 (served from cache)", got-firstCalls)
	}
}

func TestHandlerServesJSON(t *testing.T) {
	f := &fakeFetcher{repos: map[string]githubclient.Repo{
		"one": repo("a", "one", "Go"), "two": repo("b", "two", "Rust"), "three": repo("c", "three", "TypeScript"),
	}}
	svc := NewService(f, threeSlugs(), time.Minute)

	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/feed", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var feed apimodel.Feed
	if err := json.Unmarshal(rec.Body.Bytes(), &feed); err != nil {
		t.Fatalf("body is not valid Feed JSON: %v", err)
	}
	if len(feed.Items) != 3 {
		t.Errorf("items = %d, want 3", len(feed.Items))
	}
}

func TestHandlerBadGatewayWhenAllFail(t *testing.T) {
	f := &fakeFetcher{errs: map[string]error{"one": errors.New("down"), "two": errors.New("down"), "three": errors.New("down")}}
	svc := NewService(f, threeSlugs(), time.Minute)

	rec := httptest.NewRecorder()
	svc.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/feed", nil))

	if rec.Code != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", rec.Code)
	}
}
