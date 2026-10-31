package crawler

import (
	"sort"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// This file defines the evidence the classifier reasons over, and the structured
// verdict it returns. The guiding rule (see the classification review,
// docs/reviews/2026-10-30-classification) is that the two must never lose the
// difference between "this repository does not do X" and "X was not in the part
// of the repository we inspected". A bounded fetch — a truncated tree, a
// depth-limited listing, a file we failed to read — is carried explicitly so an
// absent signal is reported as a limitation, not silently as a negative fact.

// Revision identifies the repository state the evidence was gathered at. Ref is
// the branch name or commit SHA that was requested; Resolved is true only when
// Ref is a verified immutable commit SHA, so a branch name is never mistaken for
// a pinned revision.
type Revision struct {
	Ref      string
	Resolved bool
}

// TreeScope records the bounds that shaped the file listing. It lets the
// classifier tell a genuine absence ("no main package in this repository") from
// an unseen region ("the listing was capped or depth-limited, so a main package
// could lie outside it"). An absence is only honest when the scope is complete.
type TreeScope struct {
	// Truncated: the listing is incomplete at some depth (GitHub truncated a very
	// large tree, or an entry cap clipped it).
	Truncated bool
	// DepthLimited: entries deeper than the configured MaxDepth were dropped, so
	// the shallow levels are complete but deeper paths were not listed.
	DepthLimited bool
	// MaxDepth is the depth bound behind DepthLimited (path segments; 0 when
	// unknown). With it, a depth-limited listing still proves absence for paths
	// at or above the bound; without it, nothing below the root is provable.
	MaxDepth int
}

// Complete reports whether the listing is whole, so an absence claim over the
// tree is defensible.
func (s TreeScope) Complete() bool { return !s.Truncated && !s.DepthLimited }

// Covers reports whether the listing is complete for paths with the given
// number of segments ("a/b.go" has 2), i.e. whether a path that deep would have
// been listed if it existed. A truncated listing covers nothing; a depth-limited
// one covers only paths at or above a known MaxDepth.
func (s TreeScope) Covers(segments int) bool {
	if s.Truncated {
		return false
	}
	if s.DepthLimited {
		return s.MaxDepth > 0 && segments <= s.MaxDepth
	}
	return true
}

// SourceFile is one file whose content the analyzer actually read. Size is the
// byte count GitHub reported (what was retrieved); Truncated is true when the
// content was clipped below Size, in which case an absence within the file (a
// token "not found") is not provable — only what is present may be cited.
type SourceFile struct {
	Path      string
	Content   string
	Size      int
	Truncated bool
}

// Unavailable is a path the analyzer wanted to read but could not, with the
// reason (not found, fetch error, over budget). An unavailable entry point is
// the canonical case where a missing signal must not become a negative verdict.
type Unavailable struct {
	Path   string
	Reason string
}

// Inspection is the bounded, provenance-carrying evidence the classifier reasons
// over. It replaces the lossy (language, paths, manifests) triple: it keeps the
// tree scope and the per-file read status, so a fetch failure, a truncated file
// or a depth-limited listing is visible to the verdict instead of silently
// reading as "feature absent".
type Inspection struct {
	Language    string        // GitHub's primary-language guess; may be empty
	Revision    Revision      // the ref the evidence was gathered at
	Paths       []string      // the file listing (blob and tree paths), original case
	Scope       TreeScope     // the bounds that shaped Paths
	Files       []SourceFile  // files whose content was read, ordered by path
	Unavailable []Unavailable // requested paths that could not be read, ordered by path
}

// NewInspection builds an Inspection from the GitHub client's own result types,
// so the acquisition path (tree + bounded contents fetch) feeds classification
// without a lossy hand-off. Files and Unavailable are sorted by path so an
// identical fetch yields an identical, deterministically ordered Inspection.
func NewInspection(language string, rev Revision, tree githubclient.FileTree, contents githubclient.FetchedContents) Inspection {
	paths := make([]string, len(tree.Entries))
	for i, e := range tree.Entries {
		paths[i] = e.Path
	}

	files := make([]SourceFile, 0, len(contents.Files))
	for _, f := range contents.Files {
		files = append(files, SourceFile{Path: f.Path, Content: f.Content, Size: f.Size, Truncated: f.Truncated})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	unavail := make([]Unavailable, 0, len(contents.Skipped))
	for _, s := range contents.Skipped {
		unavail = append(unavail, Unavailable{Path: s.Path, Reason: s.Reason})
	}
	sort.Slice(unavail, func(i, j int) bool { return unavail[i].Path < unavail[j].Path })

	return Inspection{
		Language:    language,
		Revision:    rev,
		Paths:       paths,
		Scope:       TreeScope{Truncated: tree.Truncated, DepthLimited: tree.DepthLimited, MaxDepth: tree.MaxDepth},
		Files:       files,
		Unavailable: unavail,
	}
}

// InspectionFromSources builds an Inspection from already-gathered, fully-read
// content: every entry in sources is a file whose bytes were read in full, and
// paths is the complete tree. It asserts a complete scope, so it must not be
// used to model a partial, truncated or failed fetch — use NewInspection for
// that. Files are sorted by path so the result is independent of map order.
func InspectionFromSources(language string, paths []string, sources map[string]string) Inspection {
	files := make([]SourceFile, 0, len(sources))
	for p, c := range sources {
		files = append(files, SourceFile{Path: p, Content: c, Size: len(c)})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return Inspection{
		Language: language,
		Paths:    paths,
		Scope:    TreeScope{}, // complete: synthetic content is fully read
		Files:    files,
	}
}

// Observation is one positive thing the classifier actually inspected: the real
// source Path it came from (never a nested manifest relabelled as root), the
// Detail of what was seen (the field, excerpt or construct), and optionally what
// that legitimately Supports. Keeping observations structured lets a reader audit
// the reasoning and lets a later stage render evidence without re-deriving it.
type Observation struct {
	Path     string // actual source path; empty for a whole-tree structural fact
	Detail   string // what was inspected and what it shows
	Supports string // optional: what the observation legitimately establishes
}

// VerdictStatus says whether the classifier reached a supported conclusion or
// abstained. It is internal; the presentation boundary still maps an abstention
// to apimodel.KindOther, but the status and limitations explain why.
type VerdictStatus string

const (
	// VerdictResolved: a detector found decisive, inspected evidence for the kind.
	VerdictResolved VerdictStatus = "resolved"
	// VerdictInconclusive: the evidence was absent, ambiguous, or bounded away by
	// an incomplete fetch. The Kind is KindOther and Limitations say why.
	VerdictInconclusive VerdictStatus = "inconclusive"
)

// Classification is the crawler's verdict on what kind of project a repository
// is. The kind is never asserted on its own: Observations name the inspected
// facts that produced it, Status says whether the verdict is resolved or an
// honest abstention, and Limitations record what bounded the decision (a
// truncated tree, an unread entry point, a manifest that would not parse).
type Classification struct {
	Kind         apimodel.RepoKind
	Status       VerdictStatus
	Observations []Observation
	Limitations  []string
}

// Evidence renders the structured verdict as human-readable lines, so existing
// callers and tests that want a flat []string can get one without the classifier
// storing a duplicate copy. Observations render as "path: detail" (or just the
// detail for a whole-tree fact); limitations render with a "limitation:" prefix.
func (c Classification) Evidence() []string {
	out := make([]string, 0, len(c.Observations)+len(c.Limitations))
	for _, o := range c.Observations {
		if o.Path != "" {
			out = append(out, o.Path+": "+o.Detail)
		} else {
			out = append(out, o.Detail)
		}
	}
	for _, l := range c.Limitations {
		out = append(out, "limitation: "+l)
	}
	return out
}
