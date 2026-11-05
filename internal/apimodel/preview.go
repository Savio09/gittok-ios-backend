package apimodel

// RepoKind is the normalized shape of a repository, so the app can badge it and
// choose a layout. The GitHub API and the designs use looser labels ("Web app",
// "Backend service"); the backend maps them onto this small set.
type RepoKind string

const (
	KindFrontend RepoKind = "frontend"
	KindBackend  RepoKind = "backend"
	KindLibrary  RepoKind = "library"
	KindCLI      RepoKind = "cli"
	KindMobile   RepoKind = "mobile"
	KindOther    RepoKind = "other"
)

// Valid reports whether k is a known repo kind.
func (k RepoKind) Valid() bool {
	switch k {
	case KindFrontend, KindBackend, KindLibrary, KindCLI, KindMobile, KindOther:
		return true
	default:
		return false
	}
}

// EvidenceMode marks whether an artifact reflects the repo actually running or
// was only inferred from its source. It is the schema's firmest promise: the
// app can always tell a watched demo from a diagram drawn out of the code.
type EvidenceMode string

const (
	// ObservedRunning: captured from the software actually executing.
	ObservedRunning EvidenceMode = "observed_running"
	// NotExecuted: built from source and docs; never run.
	NotExecuted EvidenceMode = "not_executed"
)

// Valid reports whether m is a known evidence mode. Empty is valid: the
// artifact makes no runtime claim at all.
func (m EvidenceMode) Valid() bool {
	switch m {
	case "", ObservedRunning, NotExecuted:
		return true
	default:
		return false
	}
}

// Media is a preview artifact — a narrated video, a muted clip, a demo. Kind is
// a free label the app maps to a player; EvidenceMode is the runtime guarantee.
type Media struct {
	Kind         string       `json:"kind"`
	EvidenceMode EvidenceMode `json:"evidence_mode,omitempty"`
	Caption      string       `json:"caption,omitempty"`
	URL          string       `json:"url,omitempty"`
}

// SignalReason explains why a repo surfaced for this reader, tying a repo
// attribute back to their profile. It drives the "Why you're seeing this" chips.
type SignalReason string

const (
	ReasonYouKnow        SignalReason = "you_know"
	ReasonYouWantToLearn SignalReason = "you_want_to_learn"
	ReasonDomainMatch    SignalReason = "domain_match"
	ReasonPopular        SignalReason = "popular"
)

// Valid reports whether r is a known signal reason.
func (r SignalReason) Valid() bool {
	switch r {
	case ReasonYouKnow, ReasonYouWantToLearn, ReasonDomainMatch, ReasonPopular:
		return true
	default:
		return false
	}
}

// Signal is one "Why you're seeing this" chip: a repo attribute (Value) and the
// profile reason it matched.
type Signal struct {
	Value  string       `json:"value"`
	Reason SignalReason `json:"reason"`
}

// Metric is a single repo health/activity fact shown in the stat grid, e.g.
// good first issues, days since last commit, contributing guide present. It is
// evidence-backed like a claim: Value may be "Unclear" with Confidence
// uncertain when the backend could not determine it.
type Metric struct {
	Key            string         `json:"key"`
	Label          string         `json:"label"`
	Value          string         `json:"value"`
	Source         *Source        `json:"source,omitempty"`
	Confidence     Confidence     `json:"confidence,omitempty"`
	EvidenceStatus EvidenceStatus `json:"evidence_status,omitempty"`
}

// FitDimension is one axis of the fit scorecard: a named dimension scored on a
// coarse 1-3 scale with a short word. The scale is intentionally blunt — the
// signal is an estimate from repo files, not a measurement.
type FitDimension struct {
	Name  string `json:"name"`
	Level int    `json:"level"` // 1-3
	Word  string `json:"word"`
}

// Fit is how a repo lines up with the reader. Summary is the one-line human
// version; Dimensions is the optional structured scorecard. Both are carried so
// each layout can use whichever it renders.
type Fit struct {
	Summary    string         `json:"summary,omitempty"`
	Dimensions []FitDimension `json:"dimensions,omitempty"`
}

// SectionKind tags a walkthrough section so the app can switch on it. The list
// of sections is variable-length (three in one design, five in another); the
// backend emits the chunks and the app lays them out.
type SectionKind string

const (
	SectionIdea     SectionKind = "idea"     // what it does
	SectionHow      SectionKind = "how"      // how it works
	SectionStack    SectionKind = "stack"    // architecture / components
	SectionActivity SectionKind = "activity" // maintenance / health
	SectionSetup    SectionKind = "setup"    // getting started
	SectionDecision SectionKind = "decision" // your call / fit
)

// Valid reports whether k is a known section kind.
func (k SectionKind) Valid() bool {
	switch k {
	case SectionIdea, SectionHow, SectionStack, SectionActivity, SectionSetup, SectionDecision:
		return true
	default:
		return false
	}
}

// Section is one chunk of the walkthrough. Body holds prose; Claims holds the
// evidence shown in the section. EvidenceMode lets a whole section (e.g. an
// architecture diagram inferred from code) carry the runtime guarantee, not
// just Media.
type Section struct {
	Kind         SectionKind  `json:"kind"`
	Title        string       `json:"title"`
	Body         string       `json:"body,omitempty"`
	Claims       []Claim      `json:"claims,omitempty"`
	EvidenceMode EvidenceMode `json:"evidence_mode,omitempty"`
}

// RepoPreview is everything the app needs to show one repository. The top-level
// fields render the card face (identity, one-liner, media, fit, metrics,
// signals); Sections carry the deeper walkthrough. No field says where anything
// is drawn.
type RepoPreview struct {
	ID    string   `json:"id"`
	Owner string   `json:"owner"`
	Name  string   `json:"name"`
	Slug  string   `json:"slug"` // "owner/name"
	Kind  RepoKind `json:"kind"`
	// KindEvidenceStatus marks a classification as inferred or undecided.
	KindEvidenceStatus EvidenceStatus `json:"kind_evidence_status,omitempty"`
	// EvidenceMode is the execution guarantee for the preview as a whole.
	EvidenceMode EvidenceMode `json:"evidence_mode,omitempty"`
	Language     string       `json:"language,omitempty"`
	Headline     string       `json:"headline,omitempty"`
	Description  string       `json:"description"`
	GitHubURL    string       `json:"github_url"`
	License      string       `json:"license,omitempty"`

	Media    *Media    `json:"media,omitempty"`
	Signals  []Signal  `json:"signals,omitempty"`
	Metrics  []Metric  `json:"metrics,omitempty"`
	Fit      *Fit      `json:"fit,omitempty"`
	Sections []Section `json:"sections,omitempty"`
}
