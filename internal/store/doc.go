// Package store persists results — built previews and (later) user profiles —
// so the feed can be served without re-analyzing a repository on every request.
// It hides the storage backend behind interfaces the rest of the system uses.
package store
