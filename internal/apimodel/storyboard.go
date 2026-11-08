package apimodel

// StoryVisual contains reviewed wording or an exact grounded source excerpt.
// Cue is a literal substring of the owning claim's Supports, never a timer.
type StoryVisual struct {
	Kind  string   `json:"kind"`
	Title string   `json:"title"`
	Lines []string `json:"lines"`
	Focus int      `json:"focus"`
	Cue   string   `json:"cue"`
}

// StoryboardBeat references exactly one accepted claim in its own section.
type StoryboardBeat struct {
	ID      string `json:"id"`
	ClaimID string `json:"claim_id"`
}

// Storyboard preserves accepted claim order without duplicating narration.
type Storyboard struct {
	Version int              `json:"version"`
	Beats   []StoryboardBeat `json:"beats"`
}
