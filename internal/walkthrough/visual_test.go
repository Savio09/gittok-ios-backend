package walkthrough_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/walkthrough"
)

func visualFixture() (walkthrough.Bundle, walkthrough.Draft, walkthrough.Review) {
	b, d, r := storyFixture()
	for i := range d.Statements {
		d.Statements[i].Visual = &apimodel.StoryVisual{Kind: "concept", Title: "Rows to report", Lines: []string{"Rows", "Report"}, Cue: strings.Fields(d.Statements[i].Text)[0]}
	}
	return b, d, r
}

func TestVisualPublicationReferencesIdentityAndOwnership(t *testing.T) {
	b, d, r := visualFixture()
	a, err := walkthrough.Accept(b, d, r)
	if err != nil {
		t.Fatal(err)
	}
	sections := a.Sections()
	for _, section := range sections {
		if section.Storyboard == nil || section.Storyboard.Version != 1 || len(section.Storyboard.Beats) != len(section.Claims) {
			t.Fatal("missing complete storyboard")
		}
		seen := map[string]bool{}
		for i, beat := range section.Storyboard.Beats {
			if beat.ID == "" || seen[beat.ID] || beat.ClaimID != section.Claims[i].ID || section.Claims[i].Visual == nil {
				t.Fatal("invalid ordered beat reference")
			}
			seen[beat.ID] = true
		}
	}
	before, _ := json.Marshal(a.Sections())
	d.Statements[0].Visual.Lines[0] = "caller mutation"
	sections[0].Claims[0].Visual.Title = "accessor mutation"
	sections[0].Claims[0].Visual.Lines[0] = "accessor mutation"
	sections[0].Storyboard.Beats[0].ClaimID = "dangling"
	after, _ := json.Marshal(a.Sections())
	if string(before) != string(after) {
		t.Fatal("accepted visual or storyboard shares mutable storage")
	}
	changed, err := walkthrough.Accept(b, d, r)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID() == changed.ID() || a.Sections()[0].ID == changed.Sections()[0].ID || a.Sections()[0].Claims[0].ID == changed.Sections()[0].Claims[0].ID {
		t.Fatal("visual facts omitted from content identity")
	}
}

func TestVisualValidationRejectsMalformedFactsAndCues(t *testing.T) {
	cases := map[string]func(*apimodel.StoryVisual){
		"unknown kind":     func(v *apimodel.StoryVisual) { v.Kind = "script" },
		"title bound":      func(v *apimodel.StoryVisual) { v.Title = strings.Repeat("界", 57) },
		"empty title":      func(v *apimodel.StoryVisual) { v.Title = "" },
		"line bound":       func(v *apimodel.StoryVisual) { v.Lines[0] = strings.Repeat("界", 81) },
		"control line":     func(v *apimodel.StoryVisual) { v.Lines[0] = "one\ntwo" },
		"empty line":       func(v *apimodel.StoryVisual) { v.Lines[0] = " " },
		"concept count":    func(v *apimodel.StoryVisual) { v.Lines = []string{"1", "2", "3", "4"} },
		"flow count":       func(v *apimodel.StoryVisual) { v.Kind = "flow"; v.Lines = []string{"one"} },
		"comparison count": func(v *apimodel.StoryVisual) { v.Kind = "comparison"; v.Lines = []string{"one"} },
		"negative focus":   func(v *apimodel.StoryVisual) { v.Focus = -1 },
		"dangling focus":   func(v *apimodel.StoryVisual) { v.Focus = 3 },
		"missing cue":      func(v *apimodel.StoryVisual) { v.Cue = "" },
		"nonliteral cue":   func(v *apimodel.StoryVisual) { v.Cue = "not spoken" },
		"long cue":         func(v *apimodel.StoryVisual) { v.Cue = strings.Repeat("x", 41) },
		"invented source":  func(v *apimodel.StoryVisual) { v.Kind = "source" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			b, d, _ := visualFixture()
			mutate(d.Statements[0].Visual)
			if walkthrough.ValidateDraft(b, d) == nil {
				t.Fatal("malformed visual accepted")
			}
		})
	}
	for _, kind := range []string{"concept", "terminal", "flow", "comparison"} {
		b, d, _ := visualFixture()
		d.Statements[0].Visual.Kind = kind
		if err := walkthrough.ValidateDraft(b, d); err != nil {
			t.Fatalf("valid %s: %v", kind, err)
		}
	}
}

func TestSourceVisualUsesActualFirstCitationLines(t *testing.T) {
	b, d, r := visualFixture()
	b.Files[0].Content = "first original line\n\tsecond original line\nthird original line\nfourth\nfifth\nsixth\nseventh\n"
	d.Statements[0].Sources[0] = walkthrough.Citation{FileID: b.Files[0].ID, StartLine: 1, EndLine: 7, Quote: b.Files[0].Content}
	d.Statements[0].Visual = &apimodel.StoryVisual{Kind: "source", Cue: strings.Fields(d.Statements[0].Text)[0], Focus: 6}
	a, err := walkthrough.Accept(b, d, r)
	if err != nil {
		t.Fatal(err)
	}
	v := a.Sections()[0].Claims[0].Visual
	if v.Title != b.Files[0].Path || len(v.Lines) != 6 || v.Lines[0] != "first original line" || v.Lines[1] != "\tsecond original line" || v.Lines[5] != "sixth" {
		t.Fatalf("source was invented, quote-only or clipped: %+v", v)
	}
	if a.Sections()[0].Claims[0].EvidenceStatus != apimodel.EvidenceInferred || a.Sections()[0].EvidenceMode != apimodel.NotExecuted {
		t.Fatal("source visual upgraded provenance")
	}
}

func TestVisualFactsRequireTheStatementReview(t *testing.T) {
	b, d, r := visualFixture()
	d.Statements[2].Visual.Lines = []string{"Invented result"}
	r.Decisions[2].Accepted = false
	if _, err := walkthrough.Accept(b, d, r); err == nil {
		t.Fatal("rejected visual fact was published")
	}
}

func TestSourceVisualRefusesClippedOrOversizedOriginalLines(t *testing.T) {
	for _, clipped := range []bool{false, true} {
		b, d, _ := visualFixture()
		b.Files[0].Content = strings.Repeat("x", 2401)
		if clipped {
			b.Files[0].Content = "clipped source"
			b.Files[0].Truncated = true
		}
		d.Statements[0].Sources[0].Quote = b.Files[0].Content[:1]
		d.Statements[0].Visual = &apimodel.StoryVisual{Kind: "source", Cue: "project"}
		if walkthrough.ValidateDraft(b, d) == nil {
			t.Fatal("unsafe original source display accepted")
		}
	}
}

func TestSourceVisualRequiresExcerptToContainEveryDisplayedOriginalLine(t *testing.T) {
	b, d, r := visualFixture()
	d.Statements[2].Visual = &apimodel.StoryVisual{Kind: "source", Cue: "aggregates"}
	// This legacy citation quotes an exact substring but omits original indentation.
	if _, err := walkthrough.Accept(b, d, r); err == nil {
		t.Fatal("source visual published original lines absent from its claim excerpt")
	}
	d.Statements[2].Visual = nil
	if _, err := walkthrough.Accept(b, d, r); err != nil {
		t.Fatalf("legacy substring citation became invalid: %v", err)
	}
	d.Statements[2].Visual = &apimodel.StoryVisual{Kind: "source", Cue: "aggregates"}
	d.Statements[2].Sources[0].Quote = "  let totals = aggregate(rows);\n"
	a, err := walkthrough.Accept(b, d, r)
	if err != nil {
		t.Fatal(err)
	}
	claim := a.Sections()[1].Claims[1]
	for _, line := range claim.Visual.Lines {
		if !strings.Contains(claim.Excerpt, line) {
			t.Fatal("published source line is absent from the original citation excerpt")
		}
	}
}

func TestSourceVisualRejectsRepeatedLinesWhenExcerptStartsAtAnotherOccurrence(t *testing.T) {
	b, d, r := visualFixture()
	b.Files[0].Content = "a\nb\na\nb\na\nb\na\nb\n"
	d.Statements[0].Sources[0] = walkthrough.Citation{FileID: b.Files[0].ID, StartLine: 1, EndLine: 8, Quote: "b\na\nb\na\nb\na\nb\n"}
	d.Statements[0].Visual = &apimodel.StoryVisual{Kind: "source", Cue: "project"}
	if _, err := walkthrough.Accept(b, d, r); err == nil {
		t.Fatal("repeated source lines concealed a mismatched excerpt prefix")
	}
}

func TestVisualWordsCountAgainstExistingTotalProseLimit(t *testing.T) {
	b, d, _ := visualFixture()
	for len(d.Statements) < 20 {
		s := d.Statements[0]
		s.ID = "more-" + string(rune('a'+len(d.Statements)))
		s.Text = strings.Repeat("t", 390)
		s.Visual = &apimodel.StoryVisual{Kind: "terminal", Title: strings.Repeat("v", 56), Lines: []string{strings.Repeat("v", 80), strings.Repeat("v", 80), strings.Repeat("v", 80), strings.Repeat("v", 80)}, Cue: "t"}
		d.Statements = append(d.Statements, s)
	}
	if walkthrough.ValidateDraft(b, d) == nil {
		t.Fatal("visual facts escaped the existing prose cap")
	}
}

func TestVisualClaimsAreUniqueAcrossTheWholeFilm(t *testing.T) {
	for _, separateStatement := range []bool{false, true} {
		name := "reused reference"
		if separateStatement {
			name = "different draft IDs with identical publication"
		}
		t.Run(name, func(t *testing.T) {
			b, d, r := visualFixture()
			id := d.Statements[0].ID
			if separateStatement {
				duplicate := d.Statements[0]
				duplicate.ID = "copy-of-purpose"
				d.Statements = append(d.Statements, duplicate)
				id = duplicate.ID
				r.Decisions = append(r.Decisions, walkthrough.Decision{StatementID: id, Accepted: true, Reason: "Same supported source statement."})
			}
			d.Scenes = append(d.Scenes, walkthrough.Scene{ID: "repeated-purpose", Role: "purpose", StatementIDs: []string{id}})
			if err := walkthrough.ValidateDraft(b, d); err != nil {
				t.Fatalf("control must reach publication: %v", err)
			}
			a, err := walkthrough.Accept(b, d, r)
			if err == nil || a.ID() != "" || len(a.Sections()) != 0 {
				t.Fatal("duplicate visual claim identity escaped across scene boundaries")
			}
			// Legacy non-visual previews retain their previous repeated-scene behavior.
			for i := range d.Statements {
				d.Statements[i].Visual = nil
			}
			if _, err := walkthrough.Accept(b, d, r); err != nil {
				t.Fatalf("legacy repeated source scene rejected: %v", err)
			}
		})
	}
}
