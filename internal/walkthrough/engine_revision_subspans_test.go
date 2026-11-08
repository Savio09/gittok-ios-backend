package walkthrough

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestFrozenRevisionAllowsOnlyInjectiveOrderedContainedSubspans(t *testing.T) {
	original := Draft{Statements: []Statement{{ID: "s", Text: "Original source-limited statement.", Sources: []Citation{{FileID: "source-1", StartLine: 105, EndLine: 112}, {FileID: "source-1", StartLine: 200, EndLine: 207}}}}}
	for _, test := range []struct {
		name    string
		sources []Citation
		allowed bool
	}{
		{"eight to four", []Citation{{FileID: "source-1", StartLine: 105, EndLine: 108}}, true},
		{"two ordered subspans", []Citation{{FileID: "source-1", StartLine: 106, EndLine: 109}, {FileID: "source-1", StartLine: 201, EndLine: 204}}, true},
		{"expanded start", []Citation{{FileID: "source-1", StartLine: 104, EndLine: 108}}, false},
		{"expanded end", []Citation{{FileID: "source-1", StartLine: 105, EndLine: 113}}, false},
		{"split original", []Citation{{FileID: "source-1", StartLine: 105, EndLine: 108}, {FileID: "source-1", StartLine: 109, EndLine: 112}}, false},
		{"duplicate subspan", []Citation{{FileID: "source-1", StartLine: 105, EndLine: 108}, {FileID: "source-1", StartLine: 105, EndLine: 108}}, false},
		{"reversed mapping", []Citation{{FileID: "source-1", StartLine: 201, EndLine: 204}, {FileID: "source-1", StartLine: 106, EndLine: 109}}, false},
		{"new source", []Citation{{FileID: "source-2", StartLine: 105, EndLine: 108}}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			revised := Draft{Statements: []Statement{{ID: "s", Text: "Narrowed statement.", Sources: test.sources}}}
			if frozenRevision(original, revised) != test.allowed {
				t.Fatalf("contained-span policy mismatch: allowed=%v", test.allowed)
			}
		})
	}
}

func TestRevisionNarrowedSpansUseFrozenFirstReviewContext(t *testing.T) {
	inv, g, draft, review := engineInvestigation(t)
	g.files["src/report.rs"] = strings.Repeat("// prior\n", 104) + g.files["src/report.rs"] + strings.Repeat("// retained context\n", 4)
	for i := 1; i < len(draft.Statements); i++ {
		draft.Statements[i].Sources = []Citation{{FileID: inv.sourceIDs["src/report.rs"], StartLine: 105, EndLine: 112}}
	}
	for _, path := range []string{"README.md", "src/report.rs"} {
		if _, err := inv.Read(path); err != nil {
			t.Fatal(err)
		}
	}
	revised := draft
	revised.Statements = append([]Statement(nil), draft.Statements...)
	for i := 1; i < len(revised.Statements); i++ {
		revised.Statements[i].Sources = []Citation{{FileID: inv.sourceIDs["src/report.rs"], StartLine: 104 + i, EndLine: 107 + i}}
	}
	revised.Statements[2].Text = "Rows reach aggregate."
	type packet struct {
		Candidate Draft          `json:"candidate"`
		Windows   []sourceWindow `json:"cited_source_context"`
	}
	var first packet
	firstBytes := 0
	calls := 0
	cfg := engineConfig()
	cfg.MaxRevisions = 1
	engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		raw, _ := io.ReadAll(r.Body)
		var request responseRequest
		if err := json.Unmarshal(raw, &request); err != nil {
			t.Fatal(err)
		}
		switch calls {
		case 1:
			return engineHTTP(t, engineCall(t, "submit_story", draft, "original-spans")), nil
		case 2:
			var message struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(request.Input[0], &message); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(message.Content), &first); err != nil {
				t.Fatal(err)
			}
			firstBytes = len(raw)
			return engineHTTP(t, engineMessage(t, revisionRejected(review))), nil
		case 3:
			return engineHTTP(t, engineCall(t, "submit_story", revised, "narrowed-spans")), nil
		case 4:
			if len(request.Input) != 1 || strings.Contains(string(raw), "PRIVATE_FIRST_REVIEW_FEEDBACK") || strings.Contains(string(raw), "prior_candidate") {
				t.Fatal("second review inherited feedback or previous candidate")
			}
			var message struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(request.Input[0], &message); err != nil {
				t.Fatal(err)
			}
			var second packet
			if err := json.Unmarshal([]byte(message.Content), &second); err != nil {
				t.Fatal(err)
			}
			if engineJSON(t, second.Windows) != engineJSON(t, first.Windows) {
				t.Fatal("narrowing regenerated overlapping source windows instead of freezing authoritative context")
			}
			if second.Candidate.Statements[1].Sources[0].EndLine != 108 || second.Candidate.Statements[2].Text != revised.Statements[2].Text || len(raw) > firstBytes+revisionReviewGrowthBytes(first.Candidate) {
				t.Fatal("narrowed candidate or future review byte bound was lost")
			}
			return engineHTTP(t, engineMessage(t, review)), nil
		default:
			t.Fatal("unexpected provider call")
			return nil, ErrProviderUnavailable
		}
	})}))
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := engine.Run(context.Background(), "contained-subspans", inv, nil)
	if err != nil || accepted.ID() == "" || calls != 4 {
		t.Fatalf("valid eight-to-four citation narrowing was withheld: %v calls=%d", err, calls)
	}
}
