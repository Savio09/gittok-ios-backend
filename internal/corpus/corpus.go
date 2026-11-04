// Package corpus holds sanitized, provenance-labelled snapshots of real
// repositories for offline classification and pipeline tests. A snapshot
// records exactly what a bounded read-only capture saw — the tree, the files the
// canonical selection fetched, any decisive extras fetched beyond it (listed and
// justified separately), and every read failure — at one resolved commit. Tests
// read snapshots from disk; they never touch the network.
package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Snapshot is one captured repository.
type Snapshot struct {
	Repo          string                     `json:"repo"`          // "owner/name"
	SourceURL     string                     `json:"source_url"`    // the repository's page
	CapturedAt    string                     `json:"captured_at"`   // UTC date of the capture
	RequestedRef  string                     `json:"requested_ref"` // the default branch at capture
	ResolvedSHA   string                     `json:"resolved_sha"`  // the commit RequestedRef pointed at; all reads used it
	Language      string                     `json:"language"`      // GitHub's primary-language guess
	Archived      bool                       `json:"archived"`
	TreeLimits    githubclient.TreeLimits    `json:"tree_limits"`
	ContentLimits githubclient.ContentLimits `json:"content_limits"`
	Tree          Tree                       `json:"tree"`
	Selected      []string                   `json:"selected"` // the canonical selection, in fetch order
	Extras        map[string]string          `json:"extras"`   // decisive files fetched beyond the selection -> why
	Files         map[string]File            `json:"files"`    // every file read: selection and extras
	Skipped       map[string]string          `json:"skipped"`  // requested paths not returned -> reason
}

// Tree is the bounded listing as GetTree returned it.
type Tree struct {
	Truncated    bool    `json:"truncated"`
	DepthLimited bool    `json:"depth_limited"`
	MaxDepth     int     `json:"max_depth"`
	Entries      []Entry `json:"entries"`
}

// Entry is one listed path; Type is "blob" or "tree".
type Entry struct {
	Path string `json:"p"`
	Type string `json:"t"`
	Size int    `json:"s,omitempty"`
}

// File is one read file, as the bounded fetch returned it.
type File struct {
	Content   string `json:"content"`
	Size      int    `json:"size"`
	Truncated bool   `json:"truncated,omitempty"`
}

// Load reads a snapshot file.
func Load(file string) (Snapshot, error) {
	var s Snapshot
	b, err := os.ReadFile(file)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("%s: %w", file, err)
	}
	return s, nil
}

// Save writes a snapshot with stable ordering, so a recapture diffs cleanly.
func (s Snapshot) Save(file string) error {
	b, err := json.MarshalIndent(s, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, append(b, '\n'), 0o644)
}

// FileTree returns the listing in the GitHub client's shape.
func (s Snapshot) FileTree() githubclient.FileTree {
	t := githubclient.FileTree{Truncated: s.Tree.Truncated, DepthLimited: s.Tree.DepthLimited, MaxDepth: s.Tree.MaxDepth}
	// cmd/corpuscapture recorded this listing from GetTree at ResolvedSHA, so
	// the request is determined by the recorded repository and commit.
	if owner, name, ok := strings.Cut(s.Repo, "/"); ok && githubclient.ValidCommitID(s.ResolvedSHA) {
		t.Request = githubclient.TreeRequestPath(owner, name, s.ResolvedSHA)
	}
	for _, e := range s.Tree.Entries {
		t.Entries = append(t.Entries, githubclient.TreeEntry{Path: e.Path, Type: e.Type, Size: e.Size})
	}
	return t
}

// Fetched returns the contents result for the given requested paths, as the
// capture observed them; a path the capture never requested is reported as
// such rather than invented.
func (s Snapshot) Fetched(paths []string) githubclient.FetchedContents {
	var out githubclient.FetchedContents
	for _, p := range paths {
		if f, ok := s.Files[p]; ok {
			out.Files = append(out.Files, githubclient.FileContent{Path: p, Content: f.Content, Size: f.Size, Truncated: f.Truncated})
		} else if r, ok := s.Skipped[p]; ok {
			out.Skipped = append(out.Skipped, githubclient.SkippedFile{Path: p, Reason: r})
		} else {
			out.Skipped = append(out.Skipped, githubclient.SkippedFile{Path: p, Reason: "not captured"})
		}
	}
	return out
}

// AllRequested lists the selection followed by the extras, without repeats.
func (s Snapshot) AllRequested() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range s.Selected {
		if !seen[p] {
			seen[p], out = true, append(out, p)
		}
	}
	extras := make([]string, 0, len(s.Extras))
	for p := range s.Extras {
		extras = append(extras, p)
	}
	sort.Strings(extras)
	for _, p := range extras {
		if !seen[p] {
			seen[p], out = true, append(out, p)
		}
	}
	return out
}

// secretPath and secretContent recognise material a snapshot must never hold.
var (
	secretPath    = regexp.MustCompile(`(?i)(^|/)(\.env(\..*)?|id_rsa|id_ed25519|.*\.pem|.*\.key|.*\.p12|credentials(\.json)?|\.npmrc|\.pypirc|\.netrc)$`)
	secretContent = regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}|AKIA[0-9A-Z]{16}|-----BEGIN [A-Z ]*PRIVATE KEY-----|xox[baprs]-[A-Za-z0-9-]{10,}`)
)

// Secretish reports whether a path or its content looks like a credential, so
// a capture withholds it instead of storing it. Example env files are allowed.
func Secretish(p, content string) bool {
	base := strings.ToLower(path.Base(p))
	if strings.HasSuffix(base, ".example") || strings.HasSuffix(base, ".sample") || strings.HasSuffix(base, ".template") {
		return secretContent.MatchString(content)
	}
	return secretPath.MatchString(p) || secretContent.MatchString(content)
}

// CheckSanitized returns an error if any stored file looks like a secret.
func (s Snapshot) CheckSanitized() error {
	for p, f := range s.Files {
		if Secretish(p, f.Content) {
			return fmt.Errorf("%s: %s looks like a credential", s.Repo, p)
		}
	}
	return nil
}
