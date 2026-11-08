package feed

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

type RepositorySearcher interface {
	SearchRecentPublicRepositories(context.Context, time.Time, int) (githubclient.RepositorySearch, error)
}
type discoveryCursor struct {
	Version int    `json:"version"`
	Date    string `json:"date"`
	Page    int    `json:"page"`
}
type discoveryCacheEntry struct {
	page apimodel.Feed
	err  error
	at   time.Time
}

var errDiscoveryCursor = errors.New("discovery cursor expired or invalid; refresh the collection")

// DiscoveryService serves a bounded, shared public discovery feed. Cached
// metadata never substitutes for the preview pipeline's fresh public check.
// Date anchors bound the query; live star-ranked pages are not a snapshot.
type DiscoveryService struct {
	search      RepositorySearcher
	ttl         time.Duration
	now         func() time.Time
	interval    time.Duration
	gate        chan struct{}
	mu          sync.Mutex
	cache       map[discoveryCursor]discoveryCacheEntry
	lastRequest time.Time // owned by gate, including failures
}

func NewDiscoveryService(search RepositorySearcher, ttl time.Duration) *DiscoveryService {
	// The anonymous GitHub search allowance is ten/minute. Public-token mode
	// can use a shorter interval via the constructor below.
	return &DiscoveryService{search: search, ttl: ttl, now: time.Now, interval: 6100 * time.Millisecond, gate: make(chan struct{}, 1), cache: make(map[discoveryCursor]discoveryCacheEntry)}
}
func NewPublicDiscoveryService(search *githubclient.Client, ttl time.Duration) *DiscoveryService {
	s := NewDiscoveryService(search, ttl)
	if search.Mode() == "public-token" {
		s.interval = 2100 * time.Millisecond
	}
	return s
}

func (s *DiscoveryService) cursor(raw string) (discoveryCursor, error) {
	today := s.now().UTC().Truncate(24 * time.Hour)
	if raw == "" {
		return discoveryCursor{1, today.Format("2006-01-02"), 1}, nil
	}
	if len(raw) > 128 {
		return discoveryCursor{}, errDiscoveryCursor
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return discoveryCursor{}, errDiscoveryCursor
	}
	var c discoveryCursor
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if d.Decode(&c) != nil || d.Decode(new(any)) != io.EOF || c.Version != 1 || c.Page < 2 || c.Page > githubclient.DiscoveryMaxPages {
		return discoveryCursor{}, errDiscoveryCursor
	}
	date, err := time.Parse("2006-01-02", c.Date)
	if err != nil || date.After(today) || date.Before(today.AddDate(0, 0, -1)) {
		return discoveryCursor{}, errDiscoveryCursor
	}
	return c, nil
}
func (s *DiscoveryService) cached(c discoveryCursor) (apimodel.Feed, error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.cache[c]
	ttl := s.ttl
	if e.err != nil {
		ttl = 10 * time.Second
	}
	if ok && s.now().Sub(e.at) < ttl {
		return e.page, e.err, true
	}
	return apimodel.Feed{}, nil, false
}
func (s *DiscoveryService) Feed(ctx context.Context, raw string) (apimodel.Feed, error) {
	c, err := s.cursor(raw)
	if err != nil {
		return apimodel.Feed{}, err
	}
	if page, err, ok := s.cached(c); ok {
		return page, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return apimodel.Feed{}, ctx.Err()
	}
	if page, err, ok := s.cached(c); ok {
		return page, err
	}
	if wait := s.interval - time.Since(s.lastRequest); wait > 0 {
		timer := time.NewTimer(wait)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return apimodel.Feed{}, ctx.Err()
		}
	}
	s.lastRequest = time.Now()
	date, _ := time.Parse("2006-01-02", c.Date)
	result, err := s.search.SearchRecentPublicRepositories(ctx, date, c.Page)
	if err != nil || result.Incomplete || result.TotalCount < 0 || len(result.Repos) > githubclient.DiscoveryPageSize {
		failure := errors.New("public discovery temporarily unavailable")
		if ctx.Err() == nil {
			s.remember(c, discoveryCacheEntry{err: failure, at: s.now()})
		}
		return apimodel.Feed{}, failure
	}
	page := apimodel.Feed{Items: make([]apimodel.RepoPreview, 0, len(result.Repos))}
	seen := make(map[int64]bool)
	for _, r := range result.Repos {
		if !r.Public || r.Fork || r.Archived || r.ID <= 0 || r.Owner == "" || r.Name == "" || r.FullName != r.Owner+"/"+r.Name || seen[r.ID] {
			continue
		}
		seen[r.ID] = true
		page.Items = append(page.Items, preview.FromMetadata(r))
	}
	// Advance using raw search counts, not the number that survived public checks.
	if len(result.Repos) == githubclient.DiscoveryPageSize && c.Page < githubclient.DiscoveryMaxPages && c.Page*githubclient.DiscoveryPageSize < result.TotalCount {
		next := c
		next.Page++
		b, _ := json.Marshal(next)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	s.remember(c, discoveryCacheEntry{page: page, at: s.now()})
	return page, nil
}

func (s *DiscoveryService) remember(c discoveryCursor, entry discoveryCacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// At most today's and yesterday's forty pages. Remove expired entries too.
	for key, e := range s.cache {
		if s.now().Sub(e.at) >= s.ttl || key.Date < s.now().UTC().AddDate(0, 0, -1).Format("2006-01-02") {
			delete(s.cache, key)
		}
	}
	s.cache[c] = entry
}

func (s *DiscoveryService) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := s.Feed(r.Context(), r.URL.Query().Get("cursor"))
		if errors.Is(err, errDiscoveryCursor) {
			writeJSON(w, 400, map[string]string{"error": errDiscoveryCursor.Error()})
			return
		}
		if err != nil {
			w.Header().Set("Retry-After", "10")
			writeJSON(w, 503, map[string]string{"error": "Public discovery is temporarily unavailable. Try again shortly."})
			return
		}
		writeJSON(w, 200, page)
	}
}
