package githubclient

import (
	"context"
	"net/url"
	"time"
)

// Repo is the subset of GitHub repository metadata the backend uses. It is a
// stable, typed shape; callers never see GitHub's raw JSON.
type Repo struct {
	Owner       string
	Name        string
	FullName    string // "owner/name"
	Description string
	Language    string // primary language, may be empty
	Stars       int
	License     string    // SPDX id (e.g. "MIT"), or license name, or empty
	HTMLURL     string    // the repo's github.com page
	PushedAt    time.Time // last push (proxy for last-commit activity)
	Archived    bool
	Fork        bool
}

// repoPayload mirrors the GitHub /repos/{owner}/{repo} JSON we read.
type repoPayload struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
	} `json:"owner"`
	Description     string `json:"description"`
	Language        string `json:"language"`
	StargazersCount int    `json:"stargazers_count"`
	License         *struct {
		SPDXID string `json:"spdx_id"`
		Name   string `json:"name"`
	} `json:"license"`
	HTMLURL  string    `json:"html_url"`
	PushedAt time.Time `json:"pushed_at"`
	Archived bool      `json:"archived"`
	Fork     bool      `json:"fork"`
}

// GetRepo fetches metadata for a single repository. It returns ErrNotFound when
// the repo does not exist or is private to the token.
func (c *Client) GetRepo(ctx context.Context, owner, name string) (Repo, error) {
	var p repoPayload
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name)
	if err := c.get(ctx, path, &p); err != nil {
		return Repo{}, err
	}
	return Repo{
		Owner:       p.Owner.Login,
		Name:        p.Name,
		FullName:    p.FullName,
		Description: p.Description,
		Language:    p.Language,
		Stars:       p.StargazersCount,
		License:     licenseLabel(p.License),
		HTMLURL:     p.HTMLURL,
		PushedAt:    p.PushedAt,
		Archived:    p.Archived,
		Fork:        p.Fork,
	}, nil
}

// licenseLabel returns the SPDX id (e.g. "MIT"). GitHub reports "NOASSERTION"
// (with name "Other") for licenses it cannot identify; treat that as no
// recognizable license rather than surfacing the unhelpful "Other".
func licenseLabel(l *struct {
	SPDXID string `json:"spdx_id"`
	Name   string `json:"name"`
}) string {
	if l == nil || l.SPDXID == "" || l.SPDXID == "NOASSERTION" {
		return ""
	}
	return l.SPDXID
}
