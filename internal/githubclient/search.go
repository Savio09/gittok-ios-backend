package githubclient

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"time"
)

const DiscoveryPageSize = 25
const DiscoveryMaxPages = 40 // GitHub search exposes at most 1,000 results.

type RepositorySearch struct {
	Repos      []Repo
	TotalCount int
	Incomplete bool
}

// SearchRecentPublicRepositories selects popular new public projects, not
// measured star growth. The caller paces and caches these requests. No hidden
// retries are made against GitHub's separate search quota.
// https://docs.github.com/en/rest/search/search#search-repositories
func (c *Client) SearchRecentPublicRepositories(ctx context.Context, asOf time.Time, page int) (RepositorySearch, error) {
	if page < 1 || page > DiscoveryMaxPages || asOf.IsZero() {
		return RepositorySearch{}, errors.New("invalid discovery page")
	}
	end := asOf.UTC()
	q := url.Values{"q": {"is:public fork:false archived:false stars:>=20 created:" + end.AddDate(0, 0, -30).Format("2006-01-02") + ".." + end.Format("2006-01-02")}, "sort": {"stars"}, "order": {"desc"}, "per_page": {strconv.Itoa(DiscoveryPageSize)}, "page": {strconv.Itoa(page)}}
	return c.searchRepositories(ctx, q)
}

func (c *Client) searchRepositories(ctx context.Context, q url.Values) (RepositorySearch, error) {
	var payload struct {
		TotalCount *int           `json:"total_count"`
		Incomplete *bool          `json:"incomplete_results"`
		Items      *[]repoPayload `json:"items"`
	}
	oneAttempt := *c
	oneAttempt.maxRetries = 0
	if err := oneAttempt.get(ctx, "/search/repositories?"+q.Encode(), &payload); err != nil {
		return RepositorySearch{}, err
	}
	if payload.TotalCount == nil || payload.Incomplete == nil || payload.Items == nil || *payload.TotalCount < 0 || len(*payload.Items) > DiscoveryPageSize {
		return RepositorySearch{}, errors.New("invalid GitHub search response")
	}
	result := RepositorySearch{TotalCount: *payload.TotalCount, Incomplete: *payload.Incomplete, Repos: make([]Repo, 0, len(*payload.Items))}
	for _, p := range *payload.Items {
		result.Repos = append(result.Repos, p.repo())
	}
	return result, nil
}
