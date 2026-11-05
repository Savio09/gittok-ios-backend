package apimodel

// EvidenceStatus describes how the cited evidence supports a statement at its
// stated scope. It says nothing about runtime execution or reader fit.
type EvidenceStatus string

const (
	// EvidenceSupported: the source directly establishes the scoped statement.
	// An attributed README statement supports what its author says, not that
	// the described behavior works.
	EvidenceSupported EvidenceStatus = "supported"
	// EvidenceInferred: a conclusion drawn from cited premises or heuristics.
	EvidenceInferred EvidenceStatus = "inferred"
	// EvidenceUnknown: the available evidence cannot determine the answer.
	EvidenceUnknown EvidenceStatus = "unknown"
)

// Valid accepts known statuses and empty for legacy optional fields. An empty
// field makes no evidence guarantee; clients should render it as unknown.
func (s EvidenceStatus) Valid() bool {
	switch s {
	case "", EvidenceSupported, EvidenceInferred, EvidenceUnknown:
		return true
	default:
		return false
	}
}

// Confidence is the legacy coarse assessment of support for a claim.
// The backend either stands behind a claim or flags that it could not confirm
// it. There is no false precision between the two.
type Confidence string

const (
	// Confident means the cited source directly supports the claim.
	Confident Confidence = "confident"
	// Uncertain means the evidence is missing, ambiguous, or inferred, so the
	// app renders the claim as unresolved ("Not sure", "Could not determine").
	Uncertain Confidence = "uncertain"
)

// Valid reports whether c is a known confidence level.
func (c Confidence) Valid() bool {
	return c == Confident || c == Uncertain
}

// Stance is how a claim bears on the reader's fit with the repo — orthogonal to
// Confidence, which is about trust in the evidence. A claim can be confidently
// a caution, or an uncertain match. It is optional; a plain factual claim that
// is not about fit leaves it empty.
type Stance string

const (
	// Match: something the reader already knows (the "Y" mark in the designs).
	Match Stance = "match"
	// Stretch: something the reader wants to learn (the "+" mark).
	Stretch Stance = "stretch"
	// Caution: a cost or risk, e.g. heavy setup (the "~" mark).
	Caution Stance = "caution"
	// Unknown: a fit question the evidence could not answer (the "?" mark).
	Unknown Stance = "unknown"
)

// Valid reports whether s is a known stance. The empty stance is valid: it
// means the claim is not framed in terms of fit.
func (s Stance) Valid() bool {
	switch s {
	case "", Match, Stretch, Caution, Unknown:
		return true
	default:
		return false
	}
}

// SourceKind says what kind of evidence a Source points at, so the app can
// render and link it correctly. A file opens to a line range; a search shows
// the query that was run (including one that found nothing, which is how an
// uncertain claim is honestly sourced); metadata and api are repo facts and
// API responses the GitHub API reported.
type SourceKind string

const (
	SourceFile     SourceKind = "file"
	SourceSearch   SourceKind = "search"
	SourceMetadata SourceKind = "metadata"
	SourceAPI      SourceKind = "api"
)

// Valid reports whether k is a known source kind. Empty defaults to a file.
func (k SourceKind) Valid() bool {
	switch k {
	case "", SourceFile, SourceSearch, SourceMetadata, SourceAPI:
		return true
	default:
		return false
	}
}

// Source is the evidence a claim is drawn from. Ref is the file path, search
// query, or metadata key; Lines is an optional "12-18" range into a file.
type Source struct {
	Kind  SourceKind `json:"kind,omitempty"`
	Ref   string     `json:"ref"`
	Lines string     `json:"lines,omitempty"`
	// URL is a canonical source link. File URLs use the inspected commit ID;
	// metadata URLs identify the mutable provider resource, not code history.
	URL string `json:"url,omitempty"`
}

// Claim is the backbone of the schema: one evidence-linked statement about a
// repository. Every claim names its Source, explains what the source shows,
// and states the limit of what that evidence proves — so the app can show the
// reasoning next to the claim and never imply more than the evidence supports.
type Claim struct {
	// Label is the short category or headline, e.g. "Frontend is TypeScript".
	Label string `json:"label"`
	// Source is where the evidence comes from.
	Source Source `json:"source"`
	// AdditionalSources lists further evidence a multi-premise claim rests on
	// (for example, every premise of a classification). Source plus these
	// together cover the claim's whole basis; a client must render all of them.
	AdditionalSources []Source `json:"additional_sources,omitempty"`
	// Excerpt is the exact snippet (code, config, log line) the claim rests on,
	// shown verbatim. Empty when the source is a search that found nothing.
	Excerpt string `json:"excerpt,omitempty"`
	// Explanation is what the excerpt shows, in plain language.
	Explanation string `json:"explanation"`
	// Supports is what the evidence legitimately establishes.
	Supports string `json:"supports,omitempty"`
	// Limit is what it does NOT establish — e.g. "does not prove runtime
	// behavior". This is the calibrated-uncertainty guardrail in field form.
	Limit string `json:"limit,omitempty"`
	// EvidenceStatus calibrates support independently of execution and Stance.
	EvidenceStatus EvidenceStatus `json:"evidence_status,omitempty"`
	// Confidence is the legacy assessment, retained for older clients.
	Confidence Confidence `json:"confidence"`
	// Stance, when set, is how the claim bears on the reader's fit.
	Stance Stance `json:"stance,omitempty"`
}
