package feed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

type querySearchKey struct {
	Query string
	Page  int
}
type querySearchCursor struct {
	Version int    `json:"version"`
	Page    int    `json:"page"`
	Query   string `json:"query"`
}

var errSearchCursor = errors.New("this search page is invalid; search again")
var errSearchSyntax = errors.New("GitHub could not understand that search. Try a repository name or language:Swift.")

func queryDigest(q string) string { sum := sha256.Sum256([]byte(q)); return hex.EncodeToString(sum[:]) }

func searchPage(query, raw string) (int, error) {
	if raw == "" {
		return 1, nil
	}
	if len(raw) > 256 {
		return 0, errSearchCursor
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(raw)
	if err != nil {
		return 0, errSearchCursor
	}
	var c querySearchCursor
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&c) != nil || decoder.Decode(new(any)) != io.EOF || c.Version != 1 || c.Page < 2 || c.Page > githubclient.DiscoveryMaxPages || c.Query != queryDigest(query) {
		return 0, errSearchCursor
	}
	return c.Page, nil
}

func (s *DiscoveryService) cachedQuery(key querySearchKey) (apimodel.Feed, error, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.queryCache[key]
	ttl := 2 * time.Minute
	if entry.err != nil {
		ttl = 10 * time.Second
	}
	if ok && s.now().Sub(entry.at) < ttl {
		return entry.page, entry.err, true
	}
	return apimodel.Feed{}, nil, false
}

func (s *DiscoveryService) rememberQuery(key querySearchKey, entry discoveryCacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.queryCache {
		if s.now().Sub(v.at) >= 2*time.Minute {
			delete(s.queryCache, k)
		}
	}
	if len(s.queryCache) >= 32 {
		var oldest querySearchKey
		var at time.Time
		for k, v := range s.queryCache {
			if at.IsZero() || v.at.Before(at) {
				oldest = k
				at = v.at
			}
		}
		delete(s.queryCache, oldest)
	}
	s.queryCache[key] = entry
}

func (s *DiscoveryService) Search(ctx context.Context, rawQuery, rawCursor string) (apimodel.Feed, error) {
	query, err := githubclient.NormalizeRepositoryQuery(rawQuery)
	if err != nil {
		return apimodel.Feed{}, err
	}
	page, err := searchPage(query, rawCursor)
	if err != nil {
		return apimodel.Feed{}, err
	}
	key := querySearchKey{query, page}
	if err := ctx.Err(); err != nil {
		return apimodel.Feed{}, err
	}
	if result, err, ok := s.cachedQuery(key); ok {
		return result, err
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	case <-ctx.Done():
		return apimodel.Feed{}, ctx.Err()
	}
	if result, err, ok := s.cachedQuery(key); ok {
		return result, err
	}
	if err = s.waitForSearch(ctx); err != nil {
		return apimodel.Feed{}, err
	}
	found, err := s.search.SearchPublicRepositories(ctx, query, page)
	if err == nil && (found.Incomplete || found.TotalCount < 0 || len(found.Repos) > githubclient.DiscoveryPageSize) {
		err = errors.New("invalid or incomplete provider result")
	}
	if err != nil {
		var provider *githubclient.APIError
		if errors.As(err, &provider) && provider.StatusCode == 422 && strings.EqualFold(provider.Message, "Validation Failed") {
			err = errSearchSyntax
		}
		if ctx.Err() == nil {
			s.rememberQuery(key, discoveryCacheEntry{err: err, at: s.now()})
		}
		return apimodel.Feed{}, err
	}
	result := apimodel.Feed{Items: make([]apimodel.RepoPreview, 0, len(found.Repos))}
	identities := map[int64]bool{}
	slugs := map[string]bool{}
	for _, r := range found.Repos {
		slug := strings.ToLower(r.FullName)
		if !r.Public || r.ID <= 0 || r.Owner == "" || r.Name == "" || r.FullName != r.Owner+"/"+r.Name || identities[r.ID] || slugs[slug] {
			continue
		}
		identities[r.ID] = true
		slugs[slug] = true
		result.Items = append(result.Items, preview.FromMetadata(r))
	}
	if len(found.Repos) == githubclient.DiscoveryPageSize && page < githubclient.DiscoveryMaxPages && page*githubclient.DiscoveryPageSize < found.TotalCount {
		b, _ := json.Marshal(querySearchCursor{1, page + 1, queryDigest(query)})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(b)
	}
	s.rememberQuery(key, discoveryCacheEntry{page: result, at: s.now()})
	return result, nil
}

func (s *DiscoveryService) SearchHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := s.Search(r.Context(), r.URL.Query().Get("q"), r.URL.Query().Get("cursor"))
		switch {
		case err == nil:
			writeJSON(w, 200, page)
		case errors.Is(err, githubclient.ErrSearchQuery), errors.Is(err, errSearchCursor), errors.Is(err, errSearchSyntax):
			writeJSON(w, 400, map[string]string{"error": err.Error()})
		case errors.Is(err, githubclient.ErrRateLimited):
			w.Header().Set("Retry-After", "10")
			writeJSON(w, 429, map[string]string{"error": "Repository search is busy. Try again shortly."})
		default:
			w.Header().Set("Retry-After", "10")
			writeJSON(w, 503, map[string]string{"error": "Repository search is temporarily unavailable. Try again shortly."})
		}
	}
}
