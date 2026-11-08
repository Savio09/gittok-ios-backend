package walkthrough

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func productionFixture(draft *Draft, review *Review) {
	for i := range draft.Statements {
		if draft.Statements[i].Visual == nil {
			draft.Statements[i].Visual = &apimodel.StoryVisual{Kind: "concept", Title: "Rows into a report", Lines: []string{"Rows", "Report"}, Cue: strings.Fields(draft.Statements[i].Text)[0]}
		}
		if draft.Statements[i].Kind == "handoff" {
			return
		}
	}
	next := Statement{ID: "next", Kind: "handoff", Text: "Investigate how summarize receives rows at its entry.", Sources: append([]Citation(nil), draft.Statements[1].Sources...), Visual: &apimodel.StoryVisual{Kind: "flow", Title: "Investigate the entry", Lines: []string{"rows", "summarize"}, Cue: "summarize"}}
	draft.Statements = append(draft.Statements, next)
	draft.Scenes = append(draft.Scenes, Scene{ID: "next", Role: "handoff", StatementIDs: []string{"next"}})
	review.Decisions = append(review.Decisions, Decision{StatementID: "next", Accepted: true, Reason: "The inspected function entry supports this next investigation."})
}

func TestNewGenerationRequiresVisualsAndExplicitHandoffBeforeReview(t *testing.T) {
	for _, missing := range []string{"visual", "handoff"} {
		t.Run(missing, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			productionFixture(&draft, &review)
			if missing == "visual" {
				draft.Statements[0].Visual = nil
			} else {
				draft.Scenes = draft.Scenes[:len(draft.Scenes)-1]
			}
			if _, err := inv.Read("src/report.rs"); err != nil {
				t.Fatal(err)
			}
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "submit")), nil
				}
				return engineHTTP(t, engineMessage(t, review)), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			accepted, err := engine.Run(context.Background(), "requires-"+missing, inv, nil)
			if err == nil || accepted.ID() != "" || calls != 1 {
				t.Fatalf("new generation bypassed %s requirement: %v calls=%d", missing, err, calls)
			}
		})
	}
}

func TestReviewReceivesEveryPublishedVisualWordAndNestedGuardInstructions(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	productionFixture(&draft, &review)
	if err := inv.Seed(); err != nil {
		t.Fatal(err)
	}
	if _, err := inv.Read("src/report.rs"); err != nil {
		t.Fatal(err)
	}
	draft.Statements[1].Visual = &apimodel.StoryVisual{Kind: "source", Cue: "summarize"}
	bundle := inv.Snapshot()
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
		Candidate Draft `json:"candidate"`
	}
	if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
		t.Fatal(err)
	}
	v := packet.Candidate.Statements[1].Visual
	if v == nil || v.Title != "src/report.rs" || len(v.Lines) != 1 || v.Lines[0] != "fn summarize(rows: Vec<Row>) -> Report {" {
		t.Fatal("review omitted actual published source visual")
	}
	if !strings.Contains(request.Instructions, "nested") || !strings.Contains(request.Instructions, "visual") {
		t.Fatal("semantic review omitted visual facts and nested branch predicates")
	}
	if draft.Statements[1].Visual.Title != "" {
		t.Fatal("review preparation mutated writer draft")
	}
}

func TestWriterSchemaRequiresEveryVisualField(t *testing.T) {
	schema := draftSchema().(map[string]any)
	statement := schema["properties"].(map[string]any)["statements"].(map[string]any)["items"].(map[string]any)
	visual, ok := statement["properties"].(map[string]any)["visual"].(map[string]any)
	if !ok || len(visual["required"].([]string)) != 5 {
		t.Fatal("writer visual contract missing required fields")
	}
}

func TestWriterRejectsMissingNullOrUnknownVisualFields(t *testing.T) {
	for _, key := range []string{"kind", "title", "lines", "focus", "cue"} {
		for _, null := range []bool{false, true} {
			t.Run(key+map[bool]string{false: "/missing", true: "/null"}[null], func(t *testing.T) {
				wire := map[string]any{"id": "one", "kind": "purpose", "text": "Rows become a report.", "limit": "", "sources": []any{}, "visual": map[string]any{"kind": "concept", "title": "Rows", "lines": []string{"Report"}, "focus": 0, "cue": "Rows"}}
				visual := wire["visual"].(map[string]any)
				if null {
					visual[key] = nil
				} else {
					delete(visual, key)
				}
				var statement writerStatement
				if strictJSON([]byte(engineJSON(t, wire)), &statement) == nil {
					t.Fatal("incomplete visual accepted")
				}
			})
		}
	}
}

func TestGeneratedVisualsAndHandoffSurviveCompleteProviderReview(t *testing.T) {
	inv, _, draft, review := engineInvestigation(t)
	if _, err := inv.Read("src/report.rs"); err != nil {
		t.Fatal(err)
	}
	draft.Statements[1].Visual = &apimodel.StoryVisual{Kind: "terminal", Title: "Pass the rows", Lines: []string{"summarize(rows)"}, Focus: 1, Cue: "supplies rows"}
	calls := 0
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return engineHTTP(t, engineCall(t, "submit_story", draft, "visual-submit")), nil
		}
		var request responseRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		var message struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(request.Input[0], &message); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(message.Content, "summarize(rows)") || !strings.Contains(message.Content, "supplies rows") {
			t.Fatal("review omitted visual facts or narration")
		}
		return engineHTTP(t, engineMessage(t, review)), nil
	})}))
	if err != nil {
		t.Fatal(err)
	}
	a, err := engine.Run(context.Background(), "visual-reviewed", inv, nil)
	if err != nil || calls != 2 {
		t.Fatalf("valid visual generation: %v calls=%d", err, calls)
	}
	sections := a.Sections()
	if sections[len(sections)-1].Kind != apimodel.SectionHandoff || sections[len(sections)-1].Storyboard == nil {
		t.Fatal("handoff became setup or lost beats")
	}
	if sections[1].Claims[0].Visual.Lines[0] != "summarize(rows)" {
		t.Fatal("accepted visual changed")
	}
}

func TestGenerationRejectsDuplicateVisualClaimsBeforePayingReviewer(t *testing.T) {
	for _, separateStatement := range []bool{false, true} {
		name := "reused-reference"
		if separateStatement {
			name = "identical-publication"
		}
		t.Run(name, func(t *testing.T) {
			inv, _, draft, review := engineInvestigation(t)
			if _, err := inv.Read("src/report.rs"); err != nil {
				t.Fatal(err)
			}
			id := draft.Statements[0].ID
			if separateStatement {
				duplicate := draft.Statements[0]
				duplicate.ID = "duplicate-purpose"
				draft.Statements = append(draft.Statements, duplicate)
				id = duplicate.ID
				review.Decisions = append(review.Decisions, Decision{StatementID: id, Accepted: true, Reason: "Same supported content."})
			}
			draft.Scenes = append(draft.Scenes, Scene{ID: "duplicate-scene", Role: "purpose", StatementIDs: []string{id}})
			calls := 0
			engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) {
				calls++
				if calls == 1 {
					return engineHTTP(t, engineCall(t, "submit_story", draft, "submit")), nil
				}
				return engineHTTP(t, engineMessage(t, review)), nil
			})}))
			if err != nil {
				t.Fatal(err)
			}
			a, err := engine.Run(context.Background(), "unique-"+name, inv, nil)
			if err == nil || a.ID() != "" || calls != 1 {
				t.Fatalf("duplicate visual identity paid review or published: %v calls=%d", err, calls)
			}
		})
	}
}
