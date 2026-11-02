package githubclient

import (
	"context"
	"net/url"
)

// ResolveRef returns the commit SHA a ref (a branch, tag or SHA) points at now.
// Reading a tree and file contents at the returned SHA, rather than at a branch
// name, makes a capture atomic: every read sees the same immutable revision.
func (c *Client) ResolveRef(ctx context.Context, owner, name, ref string) (string, error) {
	var p struct {
		SHA string `json:"sha"`
	}
	path := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/commits/" + url.PathEscape(ref)
	if err := c.get(ctx, path, &p); err != nil {
		return "", err
	}
	return p.SHA, nil
}
