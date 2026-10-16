package apimodel

import "time"

// EventAction is a decision the reader made about a repo. These are the triage
// outcomes the whole app is built around — inspect vs skip — plus save.
type EventAction string

const (
	ActionSave    EventAction = "save"
	ActionSkip    EventAction = "skip"
	ActionInspect EventAction = "inspect"
)

// Valid reports whether a is a known action.
func (a EventAction) Valid() bool {
	switch a {
	case ActionSave, ActionSkip, ActionInspect:
		return true
	default:
		return false
	}
}

// Event is one decision the reader made, posted back to tune the feed. Reasons
// is the optional "what decided it?" tags (e.g. "stack fit", "setup too
// heavy"). At is set by the server when omitted by the client.
type Event struct {
	RepoID  string      `json:"repo_id"`
	Action  EventAction `json:"action"`
	Reasons []string    `json:"reasons,omitempty"`
	At      time.Time   `json:"at,omitempty"`
}
