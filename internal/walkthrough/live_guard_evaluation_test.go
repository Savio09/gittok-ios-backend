package walkthrough

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/generation"
)

func nestedGuardEvaluationFixture(variant string) (Bundle, Draft, error) {
	code := "func summarize(rows []int, export bool, writable bool) Report {\n  total := 0\n  for _, row := range rows { total += row }\n  report := Report{Total: total}\n  if export {\n    if writable {\n" + strings.Repeat("    // prepare export\n", 8) + "      writeReport(report)\n    }\n  }\n  return report\n}\n"
	bundle := Bundle{Identity: Identity{RepoID: 1, Owner: "example", Name: "guard-evaluation", Commit: strings.Repeat("a", 40)}, Complete: true, Files: []EvidenceFile{{ID: "code", Path: "report.go", Content: code}}}
	c := func(start, end int) []Citation { return []Citation{{FileID: "code", StartLine: start, EndLine: end}} }
	statement := func(id, kind, text string, start, end int) Statement {
		return Statement{ID: id, Kind: kind, Text: text, Sources: c(start, end), Visual: &apimodel.StoryVisual{Kind: "source", Cue: strings.Fields(text)[0]}}
	}
	draft := Draft{Statements: []Statement{
		statement("purpose", "purpose", "Summarize integer rows into a report containing their total.", 1, 4),
		statement("action", "action", "A caller supplies integer rows and export/writable flags to summarize.", 1, 1),
		statement("mechanism", "mechanism", "The loop adds each row into total and stores it in Report.Total.", 2, 4),
		statement("result", "result", "The function returns the report containing that total.", 18, 18),
		statement("export", "design", "When export is true, the function calls writeReport with the created report.", 15, 15),
		statement("handoff", "handoff", "Investigate how export and writable control the writeReport call.", 5, 6),
	}, Scenes: []Scene{{ID: "p", Role: "purpose", StatementIDs: []string{"purpose"}}, {ID: "w", Role: "workflow", StatementIDs: []string{"action", "mechanism", "result"}}, {ID: "d", Role: "design", StatementIDs: []string{"export"}}, {ID: "h", Role: "handoff", StatementIDs: []string{"handoff"}}}}
	if variant == "scoped" {
		draft.Statements[4].Text = "When export and writable are both true, the function calls writeReport with the created report."
		draft.Statements[4].Visual.Cue = "When"
	}
	draft.Statements[4].Sources = append(draft.Statements[4].Sources, c(5, 6)...)
	draft.Statements[5].Sources = append(draft.Statements[5].Sources, c(15, 15)...)

	grounded, err := groundDraft(bundle, draft)
	if err != nil {
		return Bundle{}, Draft{}, err
	}
	return bundle, grounded, validateGeneratedDraft(bundle, grounded)
}

func TestNestedGuardEvaluationControlsHaveValidEvidence(t *testing.T) {
	for _, variant := range []string{"omitted", "scoped"} {
		if _, _, err := nestedGuardEvaluationFixture(variant); err != nil {
			t.Fatalf("%s: %v", variant, err)
		}
	}
}

// This evaluation is opt-in and charges the EXISTING operator ledger. It never
// initializes accounting, retries paid calls, or fetches/executes a repository.
func TestLiveNestedGuardSemanticPair(t *testing.T) {
	if os.Getenv("GITTOK_LIVE_GUARD_EVAL") != "yes" {
		t.Skip("explicit paid semantic evaluation only")
	}
	path, output, model, key := os.Getenv("WALKTHROUGH_LEDGER_PATH"), os.Getenv("GITTOK_EVAL_OUTPUT"), os.Getenv("OPENAI_MODEL"), os.Getenv("OPENAI_API_KEY")
	if path == "" || output == "" || model == "" || key == "" {
		t.Fatal("explicit ledger/output/model/key configuration required")
	}
	if model != "gpt-6.1-sol" {
		t.Fatal("this evaluation's accounted rates are pinned to gpt-6.1-sol")
	}
	ledger, err := generation.OpenLedger(path, generation.LedgerLimits{WindowID: "capstone-2026-11-06", MaxWindowMicros: 2000000}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	engine, err := NewEngine(EngineConfig{APIKey: key, Model: model, ReasoningEffort: "low", InputMicrosPerMillion: 2500000, OutputMicrosPerMillion: 10000000, MaxCalls: 2, MaxInputBytes: 512 << 10, MaxOutputTokens: 4000, MaxJobMicros: 500000}, ledger)
	if err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{"omitted", "scoped"} {
		bundle, draft, err := nestedGuardEvaluationFixture(variant)
		if err != nil {
			t.Fatal(err)
		}
		request, err := engine.reviewRequest(bundle, draft, 1800)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		response, callErr := engine.respond(ctx, "visual-guard-v1-"+variant, &engineBudget{}, request, true)
		cancel()
		if callErr != nil {
			t.Fatal(callErr)
		}
		review, err := parseReview(response, draft)
		if err != nil {
			t.Fatal(err)
		}
		artifact := struct {
			Variant string `json:"variant"`
			Bundle  Bundle `json:"bundle"`
			Draft   Draft  `json:"draft"`
			Review  Review `json:"review"`
		}{variant, bundle, draft, review}
		raw, err := json.MarshalIndent(artifact, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(output, "guard-"+variant+".json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
		var exportAccepted bool
		for _, decision := range review.Decisions {
			if decision.StatementID == "export" {
				exportAccepted = decision.Accepted
			}
		}
		t.Logf("variant=%s whole_accepted=%t export_accepted=%t", variant, review.Accepted, exportAccepted)
		if variant == "omitted" && (review.Accepted || exportAccepted) {
			t.Error("overbroad nested-guard claim accepted")
		}
		if variant == "scoped" && (!review.Accepted || !exportAccepted) {
			t.Error("correctly scoped control rejected")
		}
	}
}
