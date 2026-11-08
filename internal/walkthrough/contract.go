// Package walkthrough gates source-grounded stories before rendering them.
// Mechanical evidence checks do not establish semantic truth: a separate
// whole-story reviewer must assess wording, component scope and direction.
package walkthrough

import (
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Identity identifies one pinned repository, independent of storage aliases.
type Identity struct {
	RepoID int64  `json:"repo_id"`
	Owner  string `json:"owner"`
	Name   string `json:"name"`
	Commit string `json:"commit"`
}

// EvidenceFile contains inspected UTF-8 source, never a merely listed path.
// Truncated content supports citations into its retained lines only. Component
// is investigator-owned scope supplied to the semantic reviewer, not a model ID.
type EvidenceFile struct {
	ID        string `json:"id"`
	Path      string `json:"path"`
	Component string `json:"component"`
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
}

// Bundle is source evidence from one investigation. Complete records acquisition
// completeness; accepting a story never upgrades it or makes partial work reusable.
type Bundle struct {
	Identity    Identity       `json:"identity"`
	Files       []EvidenceFile `json:"files"`
	Paths       []string       `json:"paths"`
	Limitations []string       `json:"limitations"`
	Complete    bool           `json:"complete"`
	CapturedAt  time.Time      `json:"captured_at"`
}

// Citation binds an exact quote to inclusive, one-based retained file lines.
type Citation struct {
	FileID    string `json:"file_id"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Quote     string `json:"quote"`
}

// Statement is the only source of factual scene prose. Kind is one of purpose,
// action, mechanism, result, design or handoff. Limit is reviewed with Text.
type Statement struct {
	ID      string                `json:"id"`
	Kind    string                `json:"kind"`
	Text    string                `json:"text"`
	Sources []Citation            `json:"sources"`
	Limit   string                `json:"limit"`
	Visual  *apimodel.StoryVisual `json:"visual,omitempty"`
}

// Scene orders statement references; it has no independent prose escape hatch.
// Role is purpose, workflow, design or handoff. IDs are draft-local references;
// publication IDs are derived from accepted content and repository identity.
type Scene struct {
	ID           string   `json:"id"`
	Role         string   `json:"role"`
	StatementIDs []string `json:"statement_ids"`
}

type Draft struct {
	Statements []Statement `json:"statements"`
	Scenes     []Scene     `json:"scenes"`
}

// Decision attests a statement including its complete premises and limit.
type Decision struct {
	StatementID string `json:"statement_id"`
	Accepted    bool   `json:"accepted"`
	Reason      string `json:"reason"`
}

// Review must come from an independent whole-story semantic reviewer. Accepted
// attests a coherent real workflow, relationship direction, component boundaries
// and source-limited wording. A positive flag alone cannot establish truth.
type Review struct {
	Accepted  bool       `json:"accepted"`
	Reason    string     `json:"reason"`
	Decisions []Decision `json:"decisions"`
}

// Accepted can only be populated by Accept. Its zero value renders no story.
// It contains no caller-owned slices or unreferenced draft statements.
type Accepted struct {
	id       string
	identity Identity
	sections []apimodel.Section
}

func (a Accepted) ID() string { return a.id }

func (a Accepted) Identity() Identity { return a.identity }

// Sections returns an independent copy, so callers cannot mutate acceptance.
func (a Accepted) Sections() []apimodel.Section {
	if a.id == "" {
		return nil
	}
	sections := append([]apimodel.Section(nil), a.sections...)
	for i := range sections {
		if sections[i].Storyboard != nil {
			storyboard := *sections[i].Storyboard
			storyboard.Beats = append([]apimodel.StoryboardBeat(nil), storyboard.Beats...)
			sections[i].Storyboard = &storyboard
		}
		sections[i].Claims = append([]apimodel.Claim(nil), sections[i].Claims...)
		for j := range sections[i].Claims {
			sections[i].Claims[j].Visual = copyVisual(sections[i].Claims[j].Visual)
			sections[i].Claims[j].AdditionalSources = append([]apimodel.Source(nil), sections[i].Claims[j].AdditionalSources...)
		}
	}
	return sections
}
