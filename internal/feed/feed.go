package feed

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
	"github.com/Savio09/gittok-ios-backend/internal/selection"
)

// RepoFetcher is the slice of the GitHub client the feed depends on. Depending
// on this interface rather than the concrete client lets the service be tested
// without a network.
type RepoFetcher interface {
	GetRepo(ctx context.Context, owner, name string) (githubclient.Repo, error)
}

// Service builds the feed from a curated set of repositories. It fetches their
// metadata from GitHub, maps each into a RepoPreview, and caches the assembled
// feed for a short time so the app can scroll without re-fetching on every
// request. It is safe for concurrent use.
type Service struct {
	fetcher RepoFetcher
	slugs   []selection.Slug
	ttl     time.Duration

	mu       sync.Mutex
	cached   apimodel.Feed
	cachedAt time.Time
}

// NewService returns a feed Service that serves slugs, caching results for ttl.
func NewService(fetcher RepoFetcher, slugs []selection.Slug, ttl time.Duration) *Service {
	return &Service{fetcher: fetcher, slugs: slugs, ttl: ttl}
}

// Feed returns the current feed, served from cache when fresh and otherwise
// rebuilt from GitHub. Repositories are fetched concurrently; the original
// curated order is preserved. If some repos fail but others succeed, the
// successes are served and the failures logged — one unreachable repo should
// not empty the feed. Only when every repo fails is an error returned.
func (s *Service) Feed(ctx context.Context) (apimodel.Feed, error) {
	s.mu.Lock()
	if len(s.cached.Items) > 0 && time.Since(s.cachedAt) < s.ttl {
		cached := s.cached
		s.mu.Unlock()
		return cached, nil
	}
	s.mu.Unlock()

	items := make([]apimodel.RepoPreview, len(s.slugs))
	errs := make([]error, len(s.slugs))
	var wg sync.WaitGroup
	for i, slug := range s.slugs {
		wg.Add(1)
		go func(i int, slug selection.Slug) {
			defer wg.Done()
			repo, err := s.fetcher.GetRepo(ctx, slug.Owner, slug.Name)
			if err != nil {
				errs[i] = err
				return
			}
			items[i] = preview.FromMetadata(repo)
		}(i, slug)
	}
	wg.Wait()

	built := make([]apimodel.RepoPreview, 0, len(s.slugs))
	var firstErr error
	for i, slug := range s.slugs {
		if errs[i] != nil {
			log.Printf("feed: skipping %s/%s: %v", slug.Owner, slug.Name, errs[i])
			if firstErr == nil {
				firstErr = errs[i]
			}
			continue
		}
		built = append(built, items[i])
	}
	if len(built) == 0 {
		return apimodel.Feed{}, firstErr
	}

	feed := apimodel.Feed{Items: built}
	s.mu.Lock()
	s.cached = feed
	s.cachedAt = time.Now()
	s.mu.Unlock()
	return feed, nil
}

// Handler serves the feed as JSON. It returns 502 when the feed could not be
// built at all (every upstream repo failed).
func (s *Service) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		feed, err := s.Feed(r.Context())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "feed temporarily unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, feed)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("feed: encode response: %v", err)
	}
}
