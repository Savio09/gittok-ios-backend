package walkthrough

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestWriterSelectorsRejectOverlongAndOverflowingRanges(t *testing.T) {
	for _, count := range []int{0, 11, 41, 54} {
		selector := writerDraft{Statements: []writerStatement{{Sources: []writerCitation{{FileID: "source-1", StartLine: 73, LineCount: count}}}}}
		if _, err := selector.toDraft(); err == nil {
			t.Fatalf("invalid selector count %d accepted", count)
		}
	}
	selector := writerDraft{Statements: []writerStatement{{Sources: []writerCitation{{FileID: "source-1", StartLine: int(^uint(0) >> 1), LineCount: 2}}}}}
	if _, err := selector.toDraft(); err == nil {
		t.Fatal("overflowing line range accepted")
	}
	selector.Statements[0].Sources[0] = writerCitation{FileID: "source-1", StartLine: 0, LineCount: 1}
	if _, err := selector.toDraft(); err == nil {
		t.Fatal("zero-based selector accepted")
	}
	selector.Statements[0].Sources[0] = writerCitation{FileID: "source-1", StartLine: 73, LineCount: 2}
	draft, err := selector.toDraft()
	if err != nil || draft.Statements[0].Sources[0].EndLine != 74 || draft.Statements[0].Sources[0].Quote != "" {
		t.Fatalf("selector conversion changed bounds or invented quote: %+v %v", draft, err)
	}
}

func TestEngineRejectsMalformedWriterSelectorsBeforePaidReview(t *testing.T) {
	for _, selection := range []map[string]any{
		{"file_id": "source-1", "start_line": 1, "line_count": 11},
		{"file_id": "source-1", "start_line": 1, "line_count": 54},
		{"file_id": "source-1", "start_line": 1, "end_line": 2},
		{"file_id": "source-1", "start_line": 1, "line_count": 2, "end_line": 2},
		{"file_id": "source-1", "start_line": 1, "line_count": 2, "quote": "model copied quote"},
	} {
		t.Run(engineJSON(t, selection), func(t *testing.T) {
			inv, _, draft, _ := engineInvestigation(t)
			for _, path := range []string{"README.md", "src/report.rs"} {
				if _, err := inv.Read(path); err != nil {
					t.Fatal(err)
				}
			}
			wire := writerDraft{Scenes: draft.Scenes, Statements: []writerStatement{{ID: "purpose", Kind: "purpose", Text: "Unapproved story", Sources: []writerCitation{}}}}
			raw := engineJSON(t, wire)
			var data map[string]any
			if err := json.Unmarshal([]byte(raw), &data); err != nil {
				t.Fatal(err)
			}
			data["statements"].([]any)[0].(map[string]any)["sources"] = []any{selection}
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				return engineHTTP(t, engineCall(t, "submit_story", data, "malformed-selector")), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "selector-invalid", inv, nil)
			if err == nil || accepted.ID() != "" || calls != 1 {
				t.Fatalf("malformed selector reached paid review: %s %v calls=%d", accepted.ID(), err, calls)
			}
		})
	}
}
