package crawler

import (
	"strconv"
	"strings"
)

// ReadState is what acquisition managed to do with one path. It is the unit the
// classifier consults before making any absence claim: only a fully read file
// can show that something is not in it, and only a covering listing can show
// that a file does not exist.
type ReadState string

const (
	StateRead        ReadState = "read"        // content consumed in full
	StateTruncated   ReadState = "truncated"   // content clipped below its size
	StateUnavailable ReadState = "unavailable" // requested but not returned
	StateListed      ReadState = "listed"      // in the listing, never fetched
	StateNotListed   ReadState = "not-listed"  // absent from the listing
)

// inspIndex is a path-keyed view of an Inspection. Building it once keeps every
// read-state question O(1) and keeps the original Inspection untouched.
type inspIndex struct {
	scope         TreeScope
	paths         []string // the listing, original order and case
	listed        map[string]bool
	files         map[string]SourceFile
	unavail       map[string]string // path -> reason
	manifestCache map[string]parsedManifest
}

func indexInspection(insp Inspection) *inspIndex {
	x := &inspIndex{
		scope:   insp.Scope,
		paths:   insp.Paths,
		listed:  make(map[string]bool, len(insp.Paths)),
		files:   make(map[string]SourceFile, len(insp.Files)),
		unavail: make(map[string]string, len(insp.Unavailable)),
	}
	for _, p := range insp.Paths {
		x.listed[p] = true
	}
	for _, f := range insp.Files {
		x.files[f.Path] = f
	}
	for _, u := range insp.Unavailable {
		x.unavail[u.Path] = u.Reason
	}
	return x
}

// state reports what happened to path p and, for an unavailable path, why.
func (x *inspIndex) state(p string) (ReadState, string) {
	if f, ok := x.files[p]; ok {
		if f.Truncated {
			return StateTruncated, ""
		}
		return StateRead, ""
	}
	if r, ok := x.unavail[p]; ok {
		return StateUnavailable, r
	}
	if x.listed[p] {
		return StateListed, ""
	}
	return StateNotListed, ""
}

// complete reports whether p was read in full — the only state in which an
// absence inside the file may be claimed.
func (x *inspIndex) complete(p string) bool {
	s, _ := x.state(p)
	return s == StateRead
}

// covers reports whether a path with the given number of segments would have
// been listed if it existed (see TreeScope.Covers).
func (x *inspIndex) covers(segments int) bool { return x.scope.Covers(segments) }

// describe renders a path's read state as a limitation naming the material gap.
func (x *inspIndex) describe(p string) string {
	switch s, reason := x.state(p); s {
	case StateTruncated:
		f := x.files[p]
		return p + " was read only partially (" + strconv.Itoa(len(f.Content)) + " of " + strconv.Itoa(f.Size) + " bytes); its unread remainder was not inspected"
	case StateUnavailable:
		return p + " could not be read (" + reason + ")"
	case StateListed:
		return p + " is listed but its content was not fetched"
	case StateNotListed:
		return p + " is not in the listing"
	default:
		return p + " was read in full"
	}
}

// segments counts path segments: "go.mod" is 1, "cmd/x/main.go" is 3.
func segments(p string) int {
	if p == "" {
		return 0
	}
	return strings.Count(p, "/") + 1
}
