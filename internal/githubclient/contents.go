package githubclient

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
)

// FileContent is one file read from a repository. Content is UTF-8 text and may
// be clipped to the per-file or budget cap, in which case Truncated is true.
// Size is the original byte size GitHub reported, before any clipping.
type FileContent struct {
	Path      string
	Content   string
	Size      int
	Truncated bool
}

// SkippedFile records a requested path that was not returned, with why.
type SkippedFile struct {
	Path   string
	Reason string
}

// ContentLimits bounds a batch fetch so a few big files cannot blow the budget.
// A non-positive value means that dimension is unbounded.
type ContentLimits struct {
	MaxFileBytes  int // per-file cap; content beyond this is truncated
	MaxTotalBytes int // cap on total content bytes across the batch
	MaxFiles      int // cap on the number of files returned
}

// DefaultContentLimits is a sensible bound for reading source files for analysis.
func DefaultContentLimits() ContentLimits {
	return ContentLimits{MaxFileBytes: 64 * 1024, MaxTotalBytes: 512 * 1024, MaxFiles: 40}
}

// FetchedContents is the result of a batch fetch: the files that were read and
// the requested paths that were skipped (with a reason). A single bad path
// never fails the whole batch.
type FetchedContents struct {
	Files   []FileContent
	Skipped []SkippedFile
}

type contentsPayload struct {
	Type     string `json:"type"`
	Encoding string `json:"encoding"`
	Size     int    `json:"size"`
	Content  string `json:"content"`
}

// GetFileContents reads the given paths from a repository at ref, bounded by
// limits. Oversized files are truncated or skipped and missing files are
// recorded rather than failing the batch, so ingestion degrades gracefully. It
// returns an error only when the context is cancelled or its deadline passes —
// immediately, even during the final request — together with the files read so
// far.
func (c *Client) GetFileContents(
	ctx context.Context,
	owner, name, ref string,
	paths []string,
	limits ContentLimits,
) (FetchedContents, error) {
	var result FetchedContents
	used := 0

	skip := func(path, reason string) {
		result.Skipped = append(result.Skipped, SkippedFile{Path: path, Reason: reason})
	}

	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if limits.MaxFiles > 0 && len(result.Files) >= limits.MaxFiles {
			skip(path, "file limit reached")
			continue
		}
		remaining := remainingBudget(limits.MaxTotalBytes, used)
		if limits.MaxTotalBytes > 0 && remaining <= 0 {
			skip(path, "content budget reached")
			continue
		}

		payload, err := c.getContents(ctx, owner, name, ref, path)
		if err != nil {
			// A cancelled or expired context ends the batch now — including on the
			// last or only path — rather than being recorded as one more skipped
			// file and noticed, if at all, on a next iteration that never comes.
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			if errors.Is(err, ErrNotFound) {
				skip(path, "not found")
			} else {
				skip(path, "fetch error: "+err.Error())
			}
			continue
		}
		if payload.Type != "file" {
			skip(path, "not a file")
			continue
		}
		if payload.Encoding != "base64" {
			skip(path, "too large to fetch inline")
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(stripWhitespace(payload.Content))
		if err != nil {
			skip(path, "decode error")
			continue
		}

		limit := perFileCap(limits.MaxFileBytes, remaining)
		content := string(decoded)
		truncated := false
		if limit >= 0 && len(content) > limit {
			content = content[:limit]
			truncated = true
		}
		used += len(content)
		result.Files = append(result.Files, FileContent{
			Path:      path,
			Content:   content,
			Size:      payload.Size,
			Truncated: truncated,
		})
	}
	return result, nil
}

func (c *Client) getContents(ctx context.Context, owner, name, ref, path string) (contentsPayload, error) {
	// Escape each path segment but keep the "/" separators GitHub expects.
	segments := strings.Split(path, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	apiPath := "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) +
		"/contents/" + strings.Join(segments, "/") + "?ref=" + url.QueryEscape(ref)

	var payload contentsPayload
	if err := c.get(ctx, apiPath, &payload); err != nil {
		return contentsPayload{}, err
	}
	return payload, nil
}

// remainingBudget is the bytes left before MaxTotalBytes; -1 means unbounded.
func remainingBudget(maxTotal, used int) int {
	if maxTotal <= 0 {
		return -1
	}
	return maxTotal - used
}

// perFileCap is the smaller of the per-file cap and the remaining budget; -1
// means unbounded. remaining is -1 when the total budget is unbounded.
func perFileCap(maxFile, remaining int) int {
	limit := -1
	if maxFile > 0 {
		limit = maxFile
	}
	if remaining >= 0 && (limit < 0 || remaining < limit) {
		limit = remaining
	}
	return limit
}

func stripWhitespace(s string) string {
	return strings.NewReplacer("\n", "", "\r", "", " ", "", "\t", "").Replace(s)
}
