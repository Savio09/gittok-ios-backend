package githubclient

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

// Repo is the subset of GitHub repository metadata the backend uses. It is a
// stable, typed shape; callers never see GitHub's raw JSON.
type Repo struct {
	Owner         string
	Name          string
	FullName      string // "owner/name"
	Description   string
	Language      string // primary language, may be empty
	Stars         int
	License       string    // SPDX id (e.g. "MIT"), or license name, or empty
	HTMLURL       string    // the repo's github.com page
	PushedAt      time.Time // last push (proxy for last-commit activity)
	Topics        []string  // repo topics, used to classify the repo
	DefaultBranch string    // e.g. "main"; the ref to fetch the file tree from
	Archived      bool
	Fork          bool
	// Public is true only when GitHub explicitly reported private:false and
	// did not report a conflicting visibility. Missing data fails closed.
	Public   bool
	Metadata map[string]MetadataField
}

// MetadataField preserves provider presence, null and scalar value separately.
// Only allowlisted facts are retained; never the complete GitHub payload.
type MetadataField struct {
	Value string
	Null  bool
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
	HTMLURL       string    `json:"html_url"`
	PushedAt      time.Time `json:"pushed_at"`
	Topics        []string  `json:"topics"`
	DefaultBranch string    `json:"default_branch"`
	Archived      bool      `json:"archived"`
	Fork          bool      `json:"fork"`
	Private       *bool     `json:"private"`
	Visibility    string    `json:"visibility"`
	metadata      map[string]MetadataField
}

func (p *repoPayload) UnmarshalJSON(data []byte) error {
	type plain repoPayload
	var value plain
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*p = repoPayload(value)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	p.metadata = make(map[string]MetadataField)
	for _, key := range []string{"full_name", "description", "language", "stargazers_count", "pushed_at", "archived", "fork"} {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		if string(raw) == "null" {
			p.metadata[key] = MetadataField{Null: true}
			continue
		}
		var scalar string
		if err := json.Unmarshal(raw, &scalar); err != nil {
			scalar = string(raw)
		}
		p.metadata[key] = MetadataField{Value: scalar}
	}
	if raw, ok := fields["license"]; ok {
		if string(raw) == "null" {
			p.metadata["license.spdx_id"] = MetadataField{Null: true}
		} else {
			var license map[string]json.RawMessage
			if err := json.Unmarshal(raw, &license); err != nil {
				return err
			}
			if raw, ok := license["spdx_id"]; ok {
				var id string
				if string(raw) == "null" {
					p.metadata["license.spdx_id"] = MetadataField{Null: true}
				} else if err := json.Unmarshal(raw, &id); err != nil {
					return err
				} else {
					p.metadata["license.spdx_id"] = MetadataField{Value: id}
				}
			}
		}
	}
	// Keep numeric evidence canonical rather than preserving insignificant JSON
	// whitespace; excerpts are intentionally never generated from parsed metadata.
	if field, ok := p.metadata["stargazers_count"]; ok && !field.Null {
		p.metadata["stargazers_count"] = MetadataField{Value: strconv.Itoa(p.StargazersCount)}
	}
	return nil
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
		Owner:         p.Owner.Login,
		Name:          p.Name,
		FullName:      p.FullName,
		Description:   p.Description,
		Language:      p.Language,
		Stars:         p.StargazersCount,
		License:       licenseLabel(p.License),
		HTMLURL:       p.HTMLURL,
		PushedAt:      p.PushedAt,
		Topics:        p.Topics,
		DefaultBranch: p.DefaultBranch,
		Archived:      p.Archived,
		Fork:          p.Fork,
		Public:        p.Private != nil && !*p.Private && (p.Visibility == "" || p.Visibility == "public"),
		Metadata:      p.metadata,
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
