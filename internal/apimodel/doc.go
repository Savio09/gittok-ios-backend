// Package apimodel defines the JSON contract exchanged between the GitTok
// backend and the SwiftUI app. It is the single source of truth for the wire
// format: the feed, repository previews, evidence-linked claims, the user
// profile, and decision events.
//
// These types are deliberately presentation-agnostic. The app has several
// candidate layouts (a full-screen reel, tap-through chapters, a triage card
// stack), but they are all renderings of the same data, so this package
// describes the data, never a layout. A screen decides how to arrange a
// RepoPreview; it never needs a field that encodes where something is drawn.
//
// The defining invariant of GitTok runs through every type here: each claim
// carries the source it was drawn from and a calibrated statement of what that
// source does and does not prove, and any artifact that could imply runtime
// behavior (a video, a diagram, a generated walkthrough) carries an
// EvidenceMode saying whether it was observed running or only inferred from
// source. The schema makes it hard to assert something the evidence doesn't
// support.
package apimodel
