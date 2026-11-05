package githubclient

import (
	"context"
	"encoding/hex"
	"fmt"
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
	if !ValidCommitID(p.SHA) {
		return "", fmt.Errorf("githubclient: invalid full commit ID")
	}
	return p.SHA, nil
}

// ValidCommitID accepts full SHA-1 and SHA-256 object IDs, never branch names
// or abbreviations. A caller must still obtain the ID by resolving a GitHub ref.
func ValidCommitID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
