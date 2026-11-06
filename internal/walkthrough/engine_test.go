package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/generation"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

type engineRoundTrip func(*http.Request) (*http.Response, error)

func (f engineRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func engineConfig() EngineConfig {
	return EngineConfig{APIKey: "explicit-test-key", Model: "explicit-test-model", InputMicrosPerMillion: 1000, OutputMicrosPerMillion: 2000, MaxCalls: 8, MaxInputBytes: 512 << 10, MaxOutputTokens: 12000, MaxJobMicros: 100000}
}

func engineLedger(t *testing.T) *generation.Ledger {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ledger.json")
	if err := os.Chmod(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	limits := generation.LedgerLimits{WindowID: "test-window", MaxWindowMicros: 1000000}
	if err := generation.InitializeLedger(path, limits, limits.WindowID); err != nil {
		t.Fatal(err)
	}
	l, err := generation.OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func engineInvestigation(t *testing.T) (*Investigation, *investigationFake, Draft, Review) {
	t.Helper()
	g := investigationFixture()
	g.files = map[string]string{"README.md": "Convert incoming rows into a report.\n", "src/report.rs": "fn summarize(rows: Vec<Row>) -> Report {\n  let totals = aggregate(rows);\n  Report::new(totals)\n}\n"}
	g.entries = nil
	for p := range g.files {
		g.entries = append(g.entries, githubclient.TreeEntry{Path: p, Type: "blob", Mode: "100644"})
	}
	inv, err := BeginInvestigation(context.Background(), g, "owner", "project", analysis.DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(inv.Close)
	id := func(p string) string { return inv.sourceIDs[p] }
	draft := Draft{Statements: []Statement{
		{ID: "p", Kind: "purpose", Text: "The project converts rows into a report.", Sources: []Citation{{FileID: id("README.md"), StartLine: 1, EndLine: 1, Quote: "Convert incoming rows into a report."}}, Limit: "Documentation describes this purpose."},
		{ID: "a", Kind: "action", Text: "A caller supplies rows to summarize.", Sources: []Citation{{FileID: id("src/report.rs"), StartLine: 1, EndLine: 1, Quote: "fn summarize(rows: Vec<Row>) -> Report {"}}},
		{ID: "m", Kind: "mechanism", Text: "The function aggregates the rows.", Sources: []Citation{{FileID: id("src/report.rs"), StartLine: 2, EndLine: 2, Quote: "let totals = aggregate(rows);"}}},
		{ID: "r", Kind: "result", Text: "A Report is constructed from the totals.", Sources: []Citation{{FileID: id("src/report.rs"), StartLine: 3, EndLine: 3, Quote: "Report::new(totals)"}}},
	}, Scenes: []Scene{{ID: "intro", Role: "purpose", StatementIDs: []string{"p"}}, {ID: "flow", Role: "workflow", StatementIDs: []string{"a", "m", "r"}}}}
	review := Review{Accepted: true, Reason: "The documented purpose and source follow one row-to-report workflow."}
	for _, s := range draft.Statements {
		review.Decisions = append(review.Decisions, Decision{StatementID: s.ID, Accepted: true, Reason: "The source supports this limited statement."})
	}
	return inv, g, draft, review
}

func engineJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func engineCall(t *testing.T, name string, args any, callID string) map[string]any {
	if name == "submit_story" {
		if draft, ok := args.(Draft); ok {
			selector := writerDraft{Scenes: draft.Scenes}
			for _, statement := range draft.Statements {
				s := writerStatement{ID: statement.ID, Kind: statement.Kind, Text: statement.Text, Limit: statement.Limit}
				for _, citation := range statement.Sources {
					s.Sources = append(s.Sources, writerCitation{FileID: citation.FileID, StartLine: citation.StartLine, LineCount: citation.EndLine - citation.StartLine + 1})
				}
				selector.Statements = append(selector.Statements, s)
			}
			args = selector
		}
	}
	return map[string]any{"type": "function_call", "name": name, "arguments": engineJSON(t, args), "call_id": callID, "status": "completed"}
}
func engineMessage(t *testing.T, value any) map[string]any {
	return map[string]any{"type": "message", "role": "assistant", "status": "completed", "content": []any{map[string]any{"type": "output_text", "text": engineJSON(t, value)}}}
}
func engineHTTP(t *testing.T, output ...any) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(engineJSON(t, map[string]any{"status": "completed", "model": "explicit-test-model", "usage": map[string]int{"input_tokens": 100, "output_tokens": 10, "total_tokens": 110}, "output": output})))}
}

func TestEngineWritesThroughBoundedToolsThenIndependentReviewer(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	ledger := engineLedger(t)
	var requests []map[string]any
	client := &http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.openai.com/v1/responses" || r.Header.Get("Authorization") != "Bearer explicit-test-key" {
			t.Fatalf("unexpected provider request: %s", r.URL)
		}
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
		if request["store"] != false || request["parallel_tool_calls"] != false || request["max_output_tokens"].(float64) > 4096 {
			t.Fatalf("unbounded request: %v", request)
		}
		if request["tool_choice"] == "required" && request["max_output_tokens"].(float64) > 1024 {
			t.Fatal("exploration consumed the final writer output reserve")
		}
		switch len(requests) {
		case 1:
			if request["tool_choice"] != "required" {
				t.Fatal("writer tool invocation was optional")
			}
			return engineHTTP(t, map[string]any{"id": "reasoning-1", "type": "reasoning", "summary": []any{}, "encrypted_content": "opaque-continuation"}, engineCall(t, "list_paths", map[string]any{"query": "src/", "limit": 10}, "c1")), nil
		case 2:
			if !strings.Contains(engineJSON(t, request["input"]), "opaque-continuation") {
				t.Fatal("reasoning continuation discarded")
			}
			return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 20}, "c2")), nil
		case 3:
			encoded := engineJSON(t, request["input"])
			if !strings.Contains(encoded, draft.Statements[1].Sources[0].FileID) || !strings.Contains(encoded, "number") {
				t.Fatal("tool output omitted real evidence ID or line numbers")
			}
			return engineHTTP(t, engineCall(t, "submit_story", draft, "c3")), nil
		case 4:
			if len(request["input"].([]any)) != 1 || request["tools"] != nil || request["text"] == nil {
				t.Fatal("reviewer inherited writer conversation or tools")
			}
			return engineHTTP(t, engineMessage(t, review)), nil
		default:
			t.Fatal("unexpected paid retry")
			return nil, errors.New("unexpected request")
		}
	})}
	engine, err := NewEngine(engineConfig(), ledger, WithHTTPClient(client))
	if err != nil {
		t.Fatal(err)
	}
	var stages []string
	accepted, err := engine.Run(context.Background(), "job-tools", inv, func(s string) { stages = append(stages, s) })
	if err != nil || accepted.ID() == "" || len(accepted.Sections()) != 2 {
		t.Fatalf("accepted=%+v error=%v", accepted, err)
	}
	if !inv.Snapshot().Complete || len(requests) != 4 || strings.Join(stages, ",") != "writing,reviewing" {
		t.Fatalf("publication gate or stages: %v %v", inv.Snapshot().Complete, stages)
	}
	if _, err := engine.Run(context.Background(), "job-tools", inv, nil); err == nil || len(requests) != 4 {
		t.Fatal("same paid generation replayed")
	}
}

func TestEngineRejectsBadCitationsBeforePayingReviewer(t *testing.T) {
	inv, _, draft, _ := engineInvestigation(t)
	draft.Statements[0].Sources[0].EndLine = 99
	calls := 0
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return engineHTTP(t, engineCall(t, "submit_story", draft, "c1")), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if a, err := engine.Run(context.Background(), "job-bad-citation", inv, nil); err == nil || a.ID() != "" || calls != 1 {
		t.Fatalf("unreviewed story: %s %v calls=%d", a.ID(), err, calls)
	}
}

func TestEngineReviewerRejectionAndIdentityChangeWithholdStory(t *testing.T) {
	for _, failure := range []string{"reviewer", "identity"} {
		t.Run(failure, func(t *testing.T) {
			inv, g, draft, review := engineInvestigation(t)
			if _, err := inv.Read("src/report.rs"); err != nil {
				t.Fatal(err)
			}
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "c1")), nil
				}
				if failure == "reviewer" {
					review.Accepted = false
				} else {
					g.repo.ID++
				}
				return engineHTTP(t, engineMessage(t, review)), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			if a, err := engine.Run(context.Background(), "job-"+failure, inv, nil); err == nil || a.ID() != "" || calls != 2 {
				t.Fatalf("story escaped gate: %s %v calls=%d", a.ID(), err, calls)
			}
		})
	}
}

func TestEngineRequiresExplicitModelPricingAndFiniteLimits(t *testing.T) {
	for _, field := range []string{"key", "model", "input price", "output price", "calls", "input", "output", "spend"} {
		t.Run(field, func(t *testing.T) {
			cfg := engineConfig()
			switch field {
			case "key":
				cfg.APIKey = ""
			case "model":
				cfg.Model = ""
			case "input price":
				cfg.InputMicrosPerMillion = 0
			case "output price":
				cfg.OutputMicrosPerMillion = 0
			case "calls":
				cfg.MaxCalls = 1
			case "input":
				cfg.MaxInputBytes = 0
			case "output":
				cfg.MaxOutputTokens = 0
			case "spend":
				cfg.MaxJobMicros = 0
			}
			if _, err := NewEngine(cfg, engineLedger(t)); err == nil {
				t.Fatal("incomplete configuration accepted")
			}
		})
	}
	if _, err := NewEngine(engineConfig(), nil); err == nil {
		t.Fatal("missing ledger accepted")
	}
}

func TestEngineClippedReviewerQuoteStopsBeforeReview(t *testing.T) {
	inv, g, draft, _ := engineInvestigation(t)
	g.files["src/report.rs"] = strings.Repeat("x", 9000) + g.files["src/report.rs"]
	if _, err := inv.Read("src/report.rs"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
		calls++
		return engineHTTP(t, engineCall(t, "submit_story", draft, "c1")), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := engine.Run(context.Background(), "job-review-clip", inv, nil); err == nil || accepted.ID() != "" || calls != 1 {
		t.Fatalf("clipped quote reached review or acceptance: %s %v calls=%d", accepted.ID(), err, calls)
	}
}

func TestEngineToolIdentityWithholdingStopsImmediately(t *testing.T) {
	inv, _, _, _ := engineInvestigation(t)
	inv.withhold()
	_, err := engineTool(inv, responseItem{Name: "read_file", Arguments: `{"path":"src/report.rs","start_line":1,"line_count":10}`})
	if !errors.Is(err, ErrIdentityWithheld) {
		t.Fatalf("identity withholding became recoverable tool error: %v", err)
	}
}

func TestEngineReviewerSeesCitedLineWhenEarlierLineRepeatsQuote(t *testing.T) {
	inv, g, draft, review := engineInvestigation(t)
	quote := "repeated quote"
	g.files["src/report.rs"] = quote + strings.Repeat("x", 9000) + "\nline2\nline3\nline4\nline5\n" + quote + "\n"
	for i := range draft.Statements {
		draft.Statements[i].Sources = []Citation{{FileID: inv.sourceIDs["src/report.rs"], StartLine: 6, EndLine: 6, Quote: quote}}
	}
	if _, err := inv.Read("src/report.rs"); err != nil {
		t.Fatal(err)
	}
	calls := 0
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return engineHTTP(t, engineCall(t, "submit_story", draft, "c1")), nil
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
			Windows []sourceWindow `json:"cited_source_context"`
		}
		if err := json.Unmarshal([]byte(request.Input[0].Content), &packet); err != nil {
			t.Fatal(err)
		}
		visible := false
		for _, window := range packet.Windows {
			for _, line := range window.Lines {
				visible = visible || line.Number == 6 && line.Text == quote
			}
		}
		if !visible {
			t.Fatal("reviewer received repeated quote from wrong line, not cited line 6")
		}
		return engineHTTP(t, engineMessage(t, review)), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	if accepted, err := engine.Run(context.Background(), "duplicate-quote", inv, nil); err != nil || accepted.ID() == "" || calls != 2 {
		t.Fatalf("exact cited context unavailable: %s %v calls=%d", accepted.ID(), err, calls)
	}
}

func TestEngineSearchFileFindsFunctionBeyondInitialWindow(t *testing.T) {
	inv, g, _, _ := engineInvestigation(t)
	g.files["src/report.rs"] = strings.Repeat("// ordinary context\n", 90) + "fn transform(rows: Vec<Row>) -> Report {\n  Report::new(rows)\n}\n"
	call := responseItem{Name: "search_file", Arguments: `{"path":"src/report.rs","query":"fn transform","start_line":1,"line_count":20}`}
	result, err := engineTool(inv, call)
	if err != nil {
		t.Fatal(err)
	}
	window, ok := result.(sourceWindow)
	if !ok || len(window.Lines) > 20 || window.FileID == "" {
		t.Fatalf("invalid search result: %+v", result)
	}
	found := false
	for _, line := range window.Lines {
		found = found || line.Number == 91 && strings.Contains(line.Text, "fn transform")
	}
	if !found {
		t.Fatal("search did not expose actual matching source line beyond initial window")
	}
	if _, err := engineTool(inv, call); err != nil || len(g.reads) != 1 {
		t.Fatalf("search renewed acquisition budget: %v %v", g.reads, err)
	}
	for _, args := range []string{`{"path":"../outside","query":"fn","start_line":1,"line_count":20}`, `{"path":"src/report.rs","query":"","start_line":1,"line_count":20}`, `{"path":"src/report.rs","query":"fn","start_line":1,"line_count":81}`} {
		if _, err := engineTool(inv, responseItem{Name: "search_file", Arguments: args}); err == nil {
			t.Fatalf("unsafe search arguments accepted: %s", args)
		}
	}
	inv.withhold()
	if _, err := engineTool(inv, call); !errors.Is(err, ErrIdentityWithheld) {
		t.Fatalf("search ignored identity withholding: %v", err)
	}
}

func TestEngineSearchPrioritizesMatchingLineAndReportsClippedMatch(t *testing.T) {
	for _, longMatch := range []bool{false, true} {
		t.Run(map[bool]string{false: "preceding long line", true: "matching long line"}[longMatch], func(t *testing.T) {
			inv, g, _, _ := engineInvestigation(t)
			if longMatch {
				g.files["src/report.rs"] = strings.Repeat("x", 9000) + "fn target()\n"
			} else {
				g.files["src/report.rs"] = strings.Repeat("x", 9000) + "\nfn target()\n"
			}
			result, err := engineTool(inv, responseItem{Name: "search_file", Arguments: `{"path":"src/report.rs","query":"target","start_line":1,"line_count":20}`})
			if err != nil {
				t.Fatal(err)
			}
			var packet struct {
				MatchedLine int          `json:"matched_line"`
				Error       string       `json:"error"`
				Lines       []sourceLine `json:"lines"`
			}
			if err := json.Unmarshal([]byte(engineJSON(t, result)), &packet); err != nil {
				t.Fatal(err)
			}
			if longMatch {
				if packet.MatchedLine != 1 || packet.Error != "matching_line_exceeds_window" || len(packet.Lines) != 0 {
					t.Fatalf("clipped match masqueraded as success: match=%d error=%q lines=%d", packet.MatchedLine, packet.Error, len(packet.Lines))
				}
			} else {
				if packet.MatchedLine != 2 || packet.Error != "" || len(packet.Lines) == 0 || packet.Lines[0].Number != 2 || !strings.Contains(packet.Lines[0].Text, "target") {
					t.Fatalf("matching line hidden behind context: match=%d error=%q lines=%d", packet.MatchedLine, packet.Error, len(packet.Lines))
				}
			}
			if len(g.reads) != 1 {
				t.Fatalf("search renewed acquisition: %v", g.reads)
			}
		})
	}
}

func TestEngineForcesFinalWriterSubmissionBeforeReviewerBudget(t *testing.T) {
	for _, pressure := range []string{"calls", "input", "cost"} {
		t.Run(pressure, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			if _, err := inv.Read("src/report.rs"); err != nil {
				t.Fatal(err)
			}
			cfg := engineConfig()
			switch pressure {
			case "calls":
				cfg.MaxCalls = 4
			case "input":
				cfg.MaxInputBytes = 32000
			case "cost":
				cfg.MaxJobMicros = 100
			}
			calls, writerCalls, forced := 0, 0, false
			engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request["text"] != nil {
					if !forced {
						t.Fatal("reviewer called without required final submission")
					}
					return engineHTTP(t, engineMessage(t, review)), nil
				}
				writerCalls++
				if !strings.Contains(request["instructions"].(string), "Remaining writer calls including this turn:") {
					t.Error("writer was not informed of trusted remaining budget")
				}
				if choice, ok := request["tool_choice"].(map[string]any); ok && choice["type"] == "function" && choice["name"] == "submit_story" {
					tools := request["tools"].([]any)
					if len(tools) != 1 || tools[0].(map[string]any)["name"] != "submit_story" {
						t.Fatal("final writer retained exploration tools")
					}
					forced = true
					return engineHTTP(t, engineCall(t, "submit_story", draft, "final-submit")), nil
				}
				return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 20}, "read-"+string(rune('a'+writerCalls)))), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "final-"+pressure, inv, nil)
			if err != nil || accepted.ID() == "" || !forced || calls > cfg.MaxCalls {
				t.Fatalf("writer exhausted before submission: accepted=%s error=%v calls=%d", accepted.ID(), err, calls)
			}
			if pressure == "calls" && calls != cfg.MaxCalls {
				t.Fatalf("last writer turn was not reserved: %d", calls)
			}
			if pressure != "calls" && writerCalls >= cfg.MaxCalls-1 {
				t.Fatal("resource pressure did not force early submission")
			}
		})
	}
}

func TestEngineReservesToolGrowthAndFinalOutputBeforeExploration(t *testing.T) {
	for _, pressure := range []string{"input growth", "final output"} {
		t.Run(pressure, func(t *testing.T) {
			inv, g, draft, review := engineInvestigation(t)
			prefix := strings.Repeat(strings.Repeat("x", 96)+"\n", 76)
			g.files["src/report.rs"] = prefix + g.files["src/report.rs"]
			for i := 1; i < len(draft.Statements); i++ {
				draft.Statements[i].Sources[0].StartLine += 76
				draft.Statements[i].Sources[0].EndLine += 76
			}
			if _, err := inv.Read("src/report.rs"); err != nil {
				t.Fatal(err)
			}
			cfg := engineConfig()
			if pressure == "input growth" {
				requestBytes := 0
				probe, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
					raw, _ := io.ReadAll(r.Body)
					requestBytes = len(raw)
					return nil, errors.New("bounded probe")
				})}))
				if err != nil {
					t.Fatal(err)
				}
				_, _ = probe.Run(context.Background(), "probe-input", inv, nil)
				if requestBytes == 0 {
					t.Fatal("probe did not reach transport")
				}
				cfg.MaxInputBytes = 2 * (2*requestBytes + 512)
			} else {
				cfg.MaxOutputTokens = 6145
			}
			calls := 0
			engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				if request["text"] != nil {
					return engineHTTP(t, engineMessage(t, review)), nil
				}
				choice, _ := request["tool_choice"].(map[string]any)
				if choice["name"] == "submit_story" {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "final-submit")), nil
				}
				if pressure == "final output" {
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(engineJSON(t, map[string]any{"status": "completed", "usage": map[string]int{"input_tokens": 100, "output_tokens": 4096, "total_tokens": 4196}, "output": []any{engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 80}, "explore")}})))}, nil
				}
				return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 80}, "explore")), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "lookahead-"+strings.ReplaceAll(pressure, " ", "-"), inv, nil)
			if err != nil || accepted.ID() == "" || calls != 2 {
				t.Fatalf("exploration consumed final submission capacity: accepted=%s err=%v calls=%d", accepted.ID(), err, calls)
			}
		})
	}
}

func TestEngineBoundsSerializedToolOutputAndContinuation(t *testing.T) {
	for _, oversized := range []string{"tool result", "continuation"} {
		t.Run(oversized, func(t *testing.T) {
			inv, g, _, _ := engineInvestigation(t)
			if oversized == "tool result" {
				g.files["src/report.rs"] = strings.Repeat("\t", 8192) + "\n"
			}
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					call := engineCall(t, "read_file", map[string]any{"path": "src/report.rs", "start_line": 1, "line_count": 80}, "read-bounded")
					if oversized == "continuation" {
						return engineHTTP(t, map[string]any{"id": "large-reasoning", "type": "reasoning", "summary": []any{}, "encrypted_content": strings.Repeat("x", 33<<10)}, call), nil
					}
					return engineHTTP(t, call), nil
				}
				var request struct {
					Input []struct {
						Type   string `json:"type"`
						Output string `json:"output"`
					} `json:"input"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Fatal(err)
				}
				output := request.Input[len(request.Input)-1].Output
				if len(output) > 16<<10 || !strings.Contains(output, "source_window_exceeds_serialized_limit") {
					t.Fatal("oversized serialized source window was carried into paid context")
				}
				return nil, errors.New("stop after bounded inspection")
			})}))
			if err != nil {
				t.Fatal(err)
			}
			_, err = engine.Run(context.Background(), "bounded-"+strings.ReplaceAll(oversized, " ", "-"), inv, nil)
			if oversized == "continuation" {
				if !errors.Is(err, ErrProviderResponse) || calls != 1 {
					t.Fatalf("oversized continuation replayed: %v calls=%d", err, calls)
				}
			} else if !errors.Is(err, ErrProviderUnavailable) || calls != 2 {
				t.Fatalf("bounded tool result: %v calls=%d", err, calls)
			}
		})
	}
}
