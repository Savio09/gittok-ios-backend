package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func distantGuardReviewFixture() (Bundle, Draft) {
	content := "if entry.IsFile && entry.Depth == 0 {\r\n" + strings.Repeat("\t// enclosing branch context\r\n", 20) + "\treadConfig(entry)\r\n" + strings.Repeat("x", 9000) + "\r\n}\r\n"
	file := EvidenceFile{ID: "guarded", Path: "src/scan.go", Component: "scanner", Content: content, Truncated: true}
	citation := Citation{FileID: file.ID, StartLine: 22, EndLine: 22, Quote: "readConfig(entry)"}
	return Bundle{Identity: Identity{RepoID: 1, Owner: "owner", Name: "project", Commit: strings.Repeat("a", 40)}, Files: []EvidenceFile{file}}, Draft{Statements: []Statement{
		{ID: "one", Kind: "mechanism", Text: "The code reads configuration.", Sources: []Citation{citation}},
		{ID: "two", Kind: "handoff", Text: "Investigate the configuration reader.", Sources: []Citation{citation}},
	}}
}

func TestIndependentReviewReceivesCompleteRetainedCitedFilesOnce(t *testing.T) {
	bundle, draft := distantGuardReviewFixture()
	request, err := (&Engine{}).reviewRequest(bundle, draft, 0)
	if err != nil {
		t.Fatal(err)
	}
	var message struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(request.Input[0], &message); err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Windows []sourceWindow `json:"cited_source_context"`
	}
	if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
		t.Fatal(err)
	}
	if len(packet.Windows) != 1 {
		t.Fatalf("each cited file must appear once, got %d windows", len(packet.Windows))
	}
	window := packet.Windows[0]
	if window.FileID != "guarded" || window.Path != "src/scan.go" || window.Component != "scanner" || !window.Truncated {
		t.Fatal("complete retained context lost source identity or honest acquisition truncation")
	}
	var text []string
	for i, line := range window.Lines {
		if line.Number != i+1 {
			t.Fatal("review context changed original line numbering")
		}
		text = append(text, line.Text)
	}
	if strings.Join(text, "\n")+"\n" != bundle.Files[0].Content {
		t.Fatal("review packet omitted distant guard or silently clipped retained source at 8KiB")
	}
	if !strings.Contains(window.Lines[0].Text, "entry.IsFile && entry.Depth == 0") || !strings.Contains(window.Lines[21].Text, "readConfig(entry)") {
		t.Fatal("material enclosing predicate was unavailable to the independent reviewer")
	}
	// Packet visibility establishes available evidence, not semantic model quality.
}

func TestCompleteReviewContextFailsExistingPreflightWithoutPaidRequest(t *testing.T) {
	for _, pressure := range []string{"input", "cost"} {
		t.Run(pressure, func(t *testing.T) {
			bundle, draft := distantGuardReviewFixture()
			bundle.Files[0].Content += strings.Repeat("// retained implementation context\n", 10000)
			cfg := engineConfig()
			if pressure == "input" {
				cfg.MaxInputBytes = 64 << 10
			} else {
				cfg.InputMicrosPerMillion = 1000000
			}
			ledger := engineLedger(t)
			calls := 0
			engine, err := NewEngine(cfg, ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) { calls++; return nil, ErrProviderUnavailable })}))
			if err != nil {
				t.Fatal(err)
			}
			request, err := engine.reviewRequest(bundle, draft, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, err = engine.respond(context.Background(), "complete-review-"+pressure, &engineBudget{}, request, true)
			if !errors.Is(err, ErrGenerationBudget) || calls != 0 {
				t.Fatalf("unaffordable complete context paid a request or omitted context: %v calls=%d", err, calls)
			}
			if _, err := ledger.Reserve("unpaid-proof", "all", 1000000); err != nil {
				t.Fatalf("failed context preflight consumed financial allowance: %v", err)
			}
		})
	}
}
