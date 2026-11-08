package walkthrough

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/analysis"
	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Scripted provider decisions test the shared contract, tools and publication
// gates. They do not establish semantic quality or real-model generalization.
func TestEngineGeneralizesSourceOnlyContractAcrossRepositoryShapes(t *testing.T) {
	fixtures := []struct {
		shape, path, source string
		text                [4]string
		pages               int
	}{
		{"frontend", "ui/compose.ts", "// Turn typed drafts into a visible preview.\nfunction onEdit(draft) {\n const preview = render(draft);\n panel.textContent = preview;\n}\n", [4]string{"The editor derives a preview from a typed draft.", "An edit supplies the draft to onEdit.", "render transforms the draft into preview text.", "The handler assigns the preview to the panel."}, 1},
		{"backend", "service/handler.go", "// Summarize readings in a JSON reply.\nfunc Handle(w Writer, readings []Reading) {\n totals := Sum(readings)\n Encode(w, totals)\n}\n", [4]string{"The handler derives a reply from readings.", "Handle receives readings and a response writer.", "Sum computes totals from the readings.", "Encode sends the totals to the writer."}, 2},
		{"cli", "tools/command.py", "\"\"\"Summarize lines from standard input.\"\"\"\ndef main(stdin):\n summary = summarize(stdin.read().splitlines())\n print(summary)\n", [4]string{"The command summarizes input lines.", "main receives an input stream.", "The command reads and splits the stream before summarizing it.", "The command prints the summary."}, 3},
		{"library", "lib/reduce.ex", "# Reduce records to labeled totals.\ndef summarize(records) do\n totals = Enum.reduce(records, %{}, &sum/2)\n {:ok, totals}\nend\n", [4]string{"The function reduces records to totals.", "summarize receives records.", "Enum.reduce accumulates records using sum.", "The function returns an ok tuple containing totals."}, 1},
		{"mobile", "Sources/ViewModel.swift", "// Derive visible items from a search query.\nfunc search(_ query: String) {\n let matches = items.filter { $0.name.contains(query) }\n self.visibleItems = matches\n}\n", [4]string{"The model derives visible items from a search query.", "search accepts the query string.", "The function filters items by matching their names.", "The matching items replace visibleItems."}, 2},
		{"mixed", "apps/web/submit.ts", "// Convert submitted rows into a displayed report.\nfunction onSubmit(rows) {\n const report = summarize(rows);\n panel.textContent = report;\n}\n", [4]string{"The web entry derives a displayed report from submitted rows.", "onSubmit receives the rows.", "The shared summarize helper reduces the submitted rows.", "The web handler assigns the report to the panel."}, 3},
		{"unknown", "c42/domain.logic", "% Relate input records to a formatted report.\nreport(Input, Output) :-\n fold_records(Input, Totals),\n format_report(Totals, Output).\n", [4]string{"The predicates relate input records to a formatted report.", "report relates its Input to an Output.", "fold_records derives Totals from Input.", "format_report derives Output from Totals."}, 1},
	}
	for index, fixture := range fixtures {
		modes := []string{"complete", "renamed"}
		if index == 0 {
			modes = append(modes, "clipped", "missing-premise", "name-only", "type-only", "dependency-only")
		}
		for _, mode := range modes {
			t.Run(fixture.shape+"/"+mode, func(t *testing.T) {
				g := investigationFixture()
				g.repo.ID, g.repo.Owner, g.repo.Name = int64(900+index), "field-lab", fmt.Sprintf("quartz-%02d-%s", index, mode)
				g.files = map[string]string{fixture.path: fixture.source} // Every fixture lacks a README and classification metadata.
				secondary := "packages/core/reduce.ts"
				if fixture.shape == "mixed" {
					g.files[secondary] = "// Reduce submitted rows.\nexport function summarize(rows) { return rows.reduce(sum); }\n"
					g.files["apps/web/package.json"], g.files["packages/core/package.json"] = "{}", "{}"
				}
				g.entries = nil
				for path := range g.files {
					g.entries = append(g.entries, githubclient.TreeEntry{Path: path, Type: "blob", Mode: "100644"})
				}
				limits := analysis.DefaultLimits()
				lines := strings.Split(fixture.source, "\n")
				if mode == "clipped" {
					limits.MaxFileBytes = len(lines[0]) + len(lines[1]) + 2
				}
				inv, err := BeginInvestigation(context.Background(), g, g.repo.Owner, g.repo.Name, limits)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(inv.Close)
				windows, calls, reviews := map[string]sourceWindow{}, 0, 0
				client := &http.Client{Transport: engineRoundTrip(func(request *http.Request) (*http.Response, error) {
					calls++
					var wire struct {
						Input      []json.RawMessage `json:"input"`
						Tools      []json.RawMessage `json:"tools"`
						Text       json.RawMessage   `json:"text"`
						ToolChoice json.RawMessage   `json:"tool_choice"`
					}
					if err := json.NewDecoder(request.Body).Decode(&wire); err != nil {
						t.Fatal(err)
					}
					if len(wire.Text) > 0 {
						reviews++
						if len(wire.Input) != 1 || len(wire.Tools) != 0 {
							t.Fatal("review inherited the writer conversation/tools")
						}
						var message struct {
							Content string `json:"content"`
						}
						if err := json.Unmarshal(wire.Input[0], &message); err != nil {
							t.Fatal(err)
						}
						var packet struct {
							Candidate Draft          `json:"candidate"`
							Windows   []sourceWindow `json:"cited_source_context"`
						}
						if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
							t.Fatal(err)
						}
						if fixture.shape == "mixed" {
							scopes := map[string]string{}
							for _, window := range packet.Windows {
								scopes[window.Path] = window.Component
							}
							if scopes[fixture.path] != "apps/web" || scopes[secondary] != "packages/core" {
								t.Fatal("review lost the cross-component boundary")
							}
						}
						review := Review{Accepted: true, Reason: "Scripted contract fixture; no semantic model evaluation."}
						for _, statement := range packet.Candidate.Statements {
							for _, citation := range statement.Sources {
								visible := false
								for _, window := range packet.Windows {
									for _, line := range window.Lines {
										// Numbered source lines omit their separator; grounded
										// single-line fixtures preserve the acquired newline.
										visible = visible || window.FileID == citation.FileID && line.Number == citation.StartLine && strings.Contains(line.Text, strings.TrimSuffix(citation.Quote, "\n"))
									}
								}
								if !visible {
									t.Fatal("review omitted an actual cited premise")
								}
							}
							review.Decisions = append(review.Decisions, Decision{StatementID: statement.ID, Accepted: true, Reason: "Scripted source-span decision."})
						}
						return engineHTTP(t, engineMessage(t, review)), nil
					}
					for _, raw := range wire.Input {
						var output struct{ Type, Output string }
						if json.Unmarshal(raw, &output) == nil && output.Type == "function_call_output" {
							var window sourceWindow
							if json.Unmarshal([]byte(output.Output), &window) == nil && window.FileID != "" {
								windows[window.Path] = window
							}
						}
					}
					if calls == 1 {
						return engineHTTP(t, engineCall(t, "list_paths", map[string]any{"query": "", "limit": 10}, "list")), nil
					}
					path := fixture.path
					if calls == 3 && fixture.shape == "mixed" {
						path = secondary
					}
					if calls == 2 || calls == 3 && fixture.shape == "mixed" {
						return engineHTTP(t, engineCall(t, "read_file", map[string]any{"path": path, "start_line": 1, "line_count": 8}, "read-"+fmt.Sprint(calls))), nil
					}
					var choice struct {
						Name string `json:"name"`
					}
					_ = json.Unmarshal(wire.ToolChoice, &choice)
					if choice.Name != "submit_story" {
						return engineHTTP(t, engineCall(t, "finish_reading", map[string]any{}, "finish")), nil
					}
					citation := func(line int) Citation {
						return Citation{FileID: windows[fixture.path].FileID, StartLine: line, EndLine: line, Quote: lines[line-1]}
					}
					draft := Draft{Scenes: []Scene{{ID: "all", Role: "workflow", StatementIDs: []string{"purpose", "action", "mechanism", "result"}}}}
					for i, kind := range []string{"purpose", "action", "mechanism", "result"} {
						draft.Statements = append(draft.Statements, Statement{ID: kind, Kind: kind, Text: fixture.text[i], Sources: []Citation{citation(i + 1)}})
					}
					if fixture.shape == "mixed" {
						draft.Statements[2].Sources = []Citation{{FileID: windows[secondary].FileID, StartLine: 2, EndLine: 2, Quote: "export function summarize(rows) { return rows.reduce(sum); }"}, citation(3)}
					}
					if fixture.pages > 1 {
						draft.Scenes = []Scene{{ID: "intro", Role: "purpose", StatementIDs: []string{"purpose"}}, {ID: "flow", Role: "workflow", StatementIDs: []string{"action", "mechanism", "result"}}}
					}
					if fixture.pages == 3 {
						draft.Statements = append(draft.Statements, Statement{ID: "detail", Kind: "design", Text: "The cited entry describes the input boundary.", Sources: []Citation{citation(2)}})
						draft.Scenes = append(draft.Scenes, Scene{ID: "detail", Role: "design", StatementIDs: []string{"detail"}})
					}
					draft.Statements = append(draft.Statements, Statement{ID: "next", Kind: "handoff", Text: "Start investigating " + fixture.path + " at the cited entry.", Sources: []Citation{citation(2)}})
					draft.Scenes = append(draft.Scenes, Scene{ID: "next", Role: "handoff", StatementIDs: []string{"next"}})
					for i := range draft.Statements {
						draft.Statements[i].Visual = &apimodel.StoryVisual{Kind: "source", Lines: []string{}, Cue: strings.Fields(draft.Statements[i].Text)[0]}
					}
					if mode == "missing-premise" {
						draft.Statements[2].Sources = nil
					}
					if strings.HasSuffix(mode, "-only") {
						draft.Statements = draft.Statements[:1]
						draft.Statements[0].Text = map[string]string{"name-only": "The project is named Quartz.", "type-only": "The project is a frontend application.", "dependency-only": "The project depends on RenderKit."}[mode]
						draft.Scenes = []Scene{{ID: "inventory", Role: "purpose", StatementIDs: []string{"purpose"}}}
					}
					return engineHTTP(t, engineCall(t, "submit_story", draft, "submit")), nil
				})}
				cfg := engineConfig()
				cfg.MaxCalls = 8
				engine, err := NewEngine(cfg, engineLedger(t), WithHTTPClient(client))
				if err != nil {
					t.Fatal(err)
				}
				accepted, err := engine.Run(context.Background(), fmt.Sprintf("matrix-%d-%s", index, mode), inv, nil)
				if mode != "complete" && mode != "renamed" {
					if err == nil || accepted.ID() != "" || reviews != 0 || calls != 4 {
						t.Fatalf("invalid premise/inventory reached paid review: %v calls=%d reviews=%d", err, calls, reviews)
					}
					return
				}
				expectedCalls := 5
				if fixture.shape == "mixed" {
					expectedCalls++
				}
				if err != nil || len(accepted.Sections()) != fixture.pages+1 || reviews != 1 || calls != expectedCalls || !inv.Snapshot().Complete {
					t.Fatalf("same pipeline rejected repository shape: %v pages=%d reviews=%d", err, len(accepted.Sections()), reviews)
				}
				for _, section := range accepted.Sections() {
					if section.Media != nil || section.EvidenceMode != apimodel.NotExecuted {
						t.Fatal("source-only story claimed execution")
					}
					for _, claim := range section.Claims {
						if claim.EvidenceStatus != apimodel.EvidenceInferred {
							t.Fatal("uncalibrated source interpretation")
						}
						for _, source := range append([]apimodel.Source{claim.Source}, claim.AdditionalSources...) {
							if source.Kind != apimodel.SourceFile || g.files[source.Ref] == "" || source.Lines == "" || !strings.Contains(source.URL, "/"+g.repo.Owner+"/"+g.repo.Name+"/blob/"+accepted.Identity().Commit+"/") {
								t.Fatal("uninspected or unpinned source")
							}
						}
					}
				}
			})
		}
	}
}
