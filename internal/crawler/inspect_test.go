package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Task 1a: every path has an explicit read state, and the listing's coverage is
// precise enough to say which absences are provable.
func TestInspectionReadStates(t *testing.T) {
	x := indexInspection(Inspection{
		Paths: []string{"go.mod", "main.go", "big.go", "gone.go", "unread.go"},
		Files: []SourceFile{
			{Path: "go.mod", Content: "module x\n", Size: 9},
			{Path: "big.go", Content: "package x\n", Size: 4096, Truncated: true},
		},
		Unavailable: []Unavailable{{Path: "gone.go", Reason: "not found"}},
	})
	cases := map[string]ReadState{
		"go.mod":    StateRead,
		"big.go":    StateTruncated,
		"gone.go":   StateUnavailable,
		"unread.go": StateListed,
		"absent.go": StateNotListed,
	}
	for p, want := range cases {
		if got, _ := x.state(p); got != want {
			t.Errorf("state(%q) = %q, want %q", p, got, want)
		}
	}
	if !x.complete("go.mod") || x.complete("big.go") {
		t.Errorf("only a fully read file is complete")
	}
	if d := x.describe("big.go"); !strings.Contains(d, "10 of 4096 bytes") {
		t.Errorf("truncation limitation should name the read region: %q", d)
	}
	if d := x.describe("gone.go"); !strings.Contains(d, "not found") {
		t.Errorf("unavailable limitation should name the reason: %q", d)
	}
}

func TestTreeScopeCovers(t *testing.T) {
	if !(TreeScope{}).Covers(40) {
		t.Errorf("a complete listing covers every depth")
	}
	if (TreeScope{Truncated: true}).Covers(1) {
		t.Errorf("a truncated listing covers nothing")
	}
	limited := TreeScope{DepthLimited: true, MaxDepth: 3}
	if !limited.Covers(3) || limited.Covers(4) {
		t.Errorf("a depth-limited listing covers exactly down to MaxDepth")
	}
	if (TreeScope{DepthLimited: true}).Covers(1) {
		t.Errorf("a depth-limited listing with an unknown bound covers nothing")
	}
}

// The applied depth bound travels from the GitHub client into the Inspection.
func TestNewInspectionCarriesDepthBound(t *testing.T) {
	insp := NewInspection("Go", Revision{Ref: "main"},
		githubclient.FileTree{DepthLimited: true, MaxDepth: 12},
		githubclient.FetchedContents{})
	if insp.Scope.MaxDepth != 12 || !insp.Scope.Covers(12) || insp.Scope.Covers(13) {
		t.Errorf("scope = %+v, want depth-limited at 12", insp.Scope)
	}
}
