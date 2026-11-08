package walkthrough

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSubmitStorySchemaUsesSourceSpansWithoutModelCopiedQuotes(t *testing.T) {
	schema := draftSchema().(map[string]any)
	statements := schema["properties"].(map[string]any)["statements"].(map[string]any)["items"].(map[string]any)
	citation := statements["properties"].(map[string]any)["sources"].(map[string]any)["items"].(map[string]any)
	properties := citation["properties"].(map[string]any)
	if len(properties) != 3 || properties["file_id"] == nil || properties["start_line"] == nil || properties["line_count"] == nil || properties["quote"] != nil || properties["end_line"] != nil {
		t.Fatal("writer selectors permit copied quotes or unconstrained end lines")
	}
	count := properties["line_count"].(map[string]any)
	if count["minimum"] != 1 || count["maximum"] != 10 {
		t.Fatal("writer citation count lacks strict 1..10 bounds")
	}
	properties = schema["properties"].(map[string]any)
	if properties["statements"].(map[string]any)["maxItems"] != maxStatements || properties["scenes"].(map[string]any)["maxItems"] != maxScenes || statements["properties"].(map[string]any)["sources"].(map[string]any)["maxItems"] != maxSources {
		t.Fatal("writer schema omits existing structural caps")
	}
}

func TestGroundDraftCopiesExactRetainedLineBytesWithoutChangingWording(t *testing.T) {
	bundle := Bundle{Files: []EvidenceFile{{ID: "source-1", Path: "src/main.go", Content: "before\r\n\tactual line\r\nlast"}}}
	draft := Draft{Statements: []Statement{{ID: "one", Text: "Keep this exact wording.", Limit: "Keep this exact limitation.", Sources: []Citation{{FileID: "source-1", StartLine: 2, EndLine: 3}}}}}
	grounded, err := groundDraft(bundle, draft)
	if err != nil {
		t.Fatal(err)
	}
	if grounded.Statements[0].Sources[0].Quote != "\tactual line\r\nlast" || grounded.Statements[0].Text != draft.Statements[0].Text || grounded.Statements[0].Limit != draft.Statements[0].Limit {
		t.Fatal("grounding changed bytes or narrative wording")
	}
	if draft.Statements[0].Sources[0].Quote != "" {
		t.Fatal("grounding mutated caller-owned draft")
	}
	for _, quote := range []string{"actual line", "wrong quote"} {
		draft.Statements[0].Sources[0].Quote = quote
		grounded, err := groundDraft(bundle, draft)
		if quote == "wrong quote" {
			if err == nil {
				t.Fatal("nonempty wrong quote was overwritten")
			}
		} else if err != nil || grounded.Statements[0].Sources[0].Quote != quote {
			t.Fatal("existing exact quote was changed")
		}
	}
}

func TestGroundDraftRejectsUnavailableClippedAndOversizedSpans(t *testing.T) {
	for _, kind := range []string{"unknown", "zero", "reversed", "out of range", "too many lines", "oversized", "invalid utf8", "clipped last line", "empty line"} {
		t.Run(kind, func(t *testing.T) {
			file := EvidenceFile{ID: "source-1", Path: "main.go", Content: "first\nlast"}
			citation := Citation{FileID: file.ID, StartLine: 1, EndLine: 1}
			switch kind {
			case "unknown":
				citation.FileID = "missing"
			case "zero":
				citation.StartLine = 0
			case "reversed":
				citation.StartLine = 2
			case "out of range":
				citation.EndLine = 3
			case "too many lines":
				file.Content = strings.Repeat("line\n", 31)
				citation.EndLine = 31
			case "oversized":
				file.Content = strings.Repeat("x", 2401)
			case "invalid utf8":
				file.Content = string([]byte{255})
			case "clipped last line":
				file.Truncated = true
				citation.StartLine = 2
				citation.EndLine = 2
			case "empty line":
				file.Content = "\n"
			}
			if _, err := groundDraft(Bundle{Files: []EvidenceFile{file}}, Draft{Statements: []Statement{{Sources: []Citation{citation}}}}); err == nil {
				t.Fatal("unsafe span was materialized")
			}
		})
	}
}

func TestEngineGroundsEmptyQuotesBeforeIndependentReview(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	for _, p := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(p); err != nil {
			t.Fatal(err)
		}
	}
	for i := range draft.Statements {
		for j := range draft.Statements[i].Sources {
			draft.Statements[i].Sources[j].Quote = ""
		}
	}
	draft.Statements[2].Sources[0].EndLine = 3
	calls := 0
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return engineHTTP(t, engineCall(t, "submit_story", draft, "submit-spans")), nil
		}
		var request struct {
			Input []struct {
				Content string `json:"content"`
			} `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var packet struct {
			Candidate Draft `json:"candidate"`
		}
		if err := json.Unmarshal([]byte(request.Input[0].Content), &packet); err != nil {
			t.Fatal(err)
		}
		if packet.Candidate.Statements[2].Sources[0].Quote != "  let totals = aggregate(rows);\n  Report::new(totals)\n" {
			t.Fatal("reviewer did not receive actual backend-extracted source bytes")
		}
		return engineHTTP(t, engineMessage(t, review)), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := engine.Run(context.Background(), "grounded-valid", inv, nil)
	if err != nil || accepted.ID() == "" || calls != 2 {
		t.Fatalf("grounded story withheld: %s %v calls=%d", accepted.ID(), err, calls)
	}
}

func TestEngineInvalidGroundingNeverPaysReviewerAndIrrelevanceStillRejects(t *testing.T) {
	for _, kind := range []string{"bad line", "unknown source", "clipped span", "irrelevant existing source"} {
		t.Run(kind, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			for _, p := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(p); err != nil {
					t.Fatal(err)
				}
			}
			for i := range draft.Statements {
				for j := range draft.Statements[i].Sources {
					draft.Statements[i].Sources[j].Quote = ""
				}
			}
			switch kind {
			case "bad line":
				draft.Statements[1].Sources[0].EndLine = 99
			case "unknown source":
				draft.Statements[1].Sources[0].FileID = "not-acquired"
			case "clipped span":
				file := inv.read["src/report.rs"]
				file.Content = "first\nclipped"
				file.Truncated = true
				inv.read[file.Path] = file
				draft.Statements[1].Sources[0].StartLine = 2
				draft.Statements[1].Sources[0].EndLine = 2
			case "irrelevant existing source":
				draft.Statements[2].Sources[0] = Citation{FileID: inv.sourceIDs["README.md"], StartLine: 1, EndLine: 1}
				review.Accepted = false
				review.Decisions[2].Accepted = false
			}
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "submit-invalid")), nil
				}
				return engineHTTP(t, engineMessage(t, review)), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "grounded-"+strings.ReplaceAll(kind, " ", "-"), inv, nil)
			wantCalls := 1
			if kind == "irrelevant existing source" {
				wantCalls = 2
			}
			if err == nil || accepted.ID() != "" || calls != wantCalls {
				t.Fatalf("citation bypassed evidence/semantic gates: %s %v calls=%d", accepted.ID(), err, calls)
			}
		})
	}
}
