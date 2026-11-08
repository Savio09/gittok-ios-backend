package githubclient

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrSearchQuery = errors.New("enter a search of 1 to 256 bytes without control characters")
var repositoryAddress = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}/[A-Za-z0-9._-]{1,100}$`)
var forkQualifier = regexp.MustCompile(`(?i)(^|\s)-?fork:(true|false|only)(\s|$)`)

// NormalizeRepositoryQuery keeps quoted terms intact and recognizes pasted repository addresses.
func NormalizeRepositoryQuery(raw string) (string, error) {
	if !utf8.ValidString(raw) || strings.IndexFunc(raw, unicode.IsControl) >= 0 {
		return "", ErrSearchQuery
	}
	q := strings.TrimSpace(raw)
	if q == "" || len(q) > 256 {
		return "", ErrSearchQuery
	}
	if u, err := url.Parse(q); err == nil && u.Scheme == "https" && strings.EqualFold(u.Host, "github.com") && u.User == nil {
		candidate := strings.TrimSuffix(strings.Trim(u.Path, "/"), ".git")
		if repositoryAddress.MatchString(candidate) {
			q = candidate
		}
	}
	if repositoryAddress.MatchString(q) {
		q = "repo:" + q
	}
	return q, nil
}

// SearchPublicRepositories uses GitHub relevance, without Discover's recency or popularity filters.
// Public reads use the existing no-scope credential policy. Search itself performs no source/model work.
// https://docs.github.com/en/rest/search/search#search-repositories
func (c *Client) SearchPublicRepositories(ctx context.Context, query string, page int) (RepositorySearch, error) {
	q, err := NormalizeRepositoryQuery(query)
	if err != nil {
		return RepositorySearch{}, err
	}
	if page < 1 || page > DiscoveryMaxPages {
		return RepositorySearch{}, ErrSearchQuery
	}
	// Include originals and forks by default; public visibility is also verified on decoded results.
	q += " is:public"
	if !hasForkQualifier(q) {
		q += " fork:true"
	}
	params := url.Values{"q": {q}, "per_page": {strconv.Itoa(DiscoveryPageSize)}, "page": {strconv.Itoa(page)}}
	return c.searchRepositories(NewRequestBudget(1, 4<<20).Context(ctx), params)
}

func hasForkQualifier(q string) bool {
	var outside strings.Builder
	quoted, escaped := false, false
	for _, r := range q {
		if r == '"' && !escaped {
			quoted = !quoted
			outside.WriteRune(' ')
		} else if !quoted {
			outside.WriteRune(r)
		}
		if r == '\\' && !escaped {
			escaped = true
		} else {
			escaped = false
		}
	}
	return forkQualifier.MatchString(outside.String())
}
