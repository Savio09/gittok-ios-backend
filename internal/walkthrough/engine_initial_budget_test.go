package walkthrough

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func budgetContextBundle() Bundle {
	return Bundle{Identity: Identity{RepoID: 1, Owner: "owner", Name: "project", Commit: strings.Repeat("a", 40)}, Files: []EvidenceFile{
		{ID: "code", Path: "main.go", Content: strings.Repeat("// product context\n", 308) + "func renderProduct() {}\n" + strings.Repeat("// more implementation\n", 170)},
		{ID: "secondary", Path: "helper.go", Content: strings.Repeat("// supporting implementation details\n", 200)},
		{ID: "doc", Path: "README.md", Content: "Render a user's input into useful output.\n" + strings.Repeat("Usage and source notes.\n", 59)},
	}}
}

func initialBudgetWindows(t *testing.T, request responseRequest) []sourceWindow {
	t.Helper()
	var message struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal(request.Input[0], &message); err != nil {
		t.Fatal(err)
	}
	var packet struct {
		Windows []sourceWindow `json:"inspected_source_windows"`
	}
	if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
		t.Fatal(err)
	}
	return packet.Windows
}

func TestInitialProjectionFitsExactFinalCostWithoutLosingCoreOrAcquiredRegistry(t *testing.T) {
	bundle := budgetContextBundle()
	before := engineJSON(t, bundle)
	cfg := engineConfig()
	cfg.MaxCalls = 8
	cfg.InputMicrosPerMillion = 2500000
	cfg.OutputMicrosPerMillion = 10000000
	cfg.MaxJobMicros = 500000
	ledger := engineLedger(t)
	engine, err := NewEngine(cfg, ledger)
	if err != nil {
		t.Fatal(err)
	}
	full, err := initialContextRequest(bundle, nil, initialEvidenceWindows(bundle.Files))
	if err != nil {
		t.Fatal(err)
	}
	final, _ := engine.writerTurn(full, &engineBudget{}, true)
	if _, err := engine.prepareRequest(&engineBudget{}, final, false); err == nil {
		t.Fatal("fixture is not an oversized initial context at configured rates")
	}
	request, err := engine.fittedInitialRequest(bundle, nil)
	if err != nil {
		t.Fatal(err)
	}
	final, _ = engine.writerTurn(request, &engineBudget{}, true)
	prepared, err := engine.prepareRequest(&engineBudget{}, final, false)
	if err != nil || prepared.maximum > 250000 {
		t.Fatalf("projection did not fit exact final preflight: %v", err)
	}
	core, docs := false, false
	for _, window := range initialBudgetWindows(t, request) {
		if window.FileID == "code" {
			core = len(window.Lines) == 479 && !window.Truncated
			found := false
			for _, line := range window.Lines {
				found = found || line.Number == 309 && strings.Contains(line.Text, "renderProduct")
			}
			core = core && found
		}
		docs = docs || window.FileID == "doc" && len(window.Lines) > 0 && strings.Contains(window.Lines[0].Text, "Render a user's input into useful output.")
	}
	if !core || !docs || engineJSON(t, bundle) != before {
		t.Fatal("fitting lost complete core, useful docs, or mutated acquired registry")
	}
	if _, err := ledger.Reserve("preflight-proof", "all", 1000000); err != nil {
		t.Fatalf("preflight mutated paid accounting: %v", err)
	}
}

func TestFittingLeavesAffordableContextUnchangedAndClipsOversizedUTF8(t *testing.T) {
	bundle := budgetContextBundle()
	engine, err := NewEngine(engineConfig(), engineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	request, err := engine.fittedInitialRequest(bundle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if engineJSON(t, initialBudgetWindows(t, request)) != engineJSON(t, initialEvidenceWindows(bundle.Files)) {
		t.Fatal("affordable context changed")
	}
	cfg := engineConfig()
	cfg.InputMicrosPerMillion = 2500000
	cfg.OutputMicrosPerMillion = 10000000
	cfg.MaxJobMicros = 500000
	engine, err = NewEngine(cfg, engineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	bundle.Files = []EvidenceFile{{ID: "large", Path: "core.go", Content: strings.Repeat("界<", 10000)}}
	request, err = engine.fittedInitialRequest(bundle, nil)
	if err != nil {
		t.Fatal(err)
	}
	windows := initialBudgetWindows(t, request)
	if len(windows) != 1 || !windows[0].Truncated || len(windows[0].Lines) != 1 || !utf8.ValidString(windows[0].Lines[0].Text) || len(windows[0].Lines[0].Text) >= 24<<10 {
		t.Fatal("oversized code was not honestly clipped at a UTF8 boundary")
	}
	cfg.MaxJobMicros = 1
	ledger := engineLedger(t)
	engine, err = NewEngine(cfg, ledger)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.fittedInitialRequest(bundle, nil); err == nil {
		t.Fatal("absurd cap admitted paid context")
	}
	if _, err := ledger.Reserve("no-paid-proof", "all", 1000000); err != nil {
		t.Fatal("failed preflight charged the ledger")
	}
}

func TestFittedContextPrefersPrimaryCodeOverLargerHelpersAndBuildConfig(t *testing.T) {
	for _, test := range []struct{ name, core, secondary string }{{"entry over helper", "main.go", "helper.go"}, {"entry over build config", "index.ts", "vite.config.ts"}, {"product over example entry", "library.go", "examples/main.go"}} {
		t.Run(test.name, func(t *testing.T) {
			bundle := budgetContextBundle()
			bundle.Files[0].Path = test.core
			bundle.Files[1].Path = test.secondary
			bundle.Files[1].Content = strings.Repeat("// secondary implementation details\n", 760)
			bundle.Files = append(bundle.Files, EvidenceFile{ID: "module", Path: "go.mod", Content: "module example.test/project\n"})
			cfg := engineConfig()
			cfg.InputMicrosPerMillion = 2500000
			cfg.OutputMicrosPerMillion = 10000000
			cfg.MaxJobMicros = 500000
			engine, err := NewEngine(cfg, engineLedger(t))
			if err != nil {
				t.Fatal(err)
			}
			request, err := engine.fittedInitialRequest(bundle, nil)
			if err != nil {
				t.Fatal(err)
			}
			windows := initialBudgetWindows(t, request)
			if len(windows) == 0 || windows[0].FileID != "code" || len(windows[0].Lines) != 479 || windows[0].Truncated {
				t.Fatal("larger helper/build/example context displaced complete primary code")
			}
		})
	}
}

func TestClippedInitialWindowReportsDroppedBlankLines(t *testing.T) {
	original := sourceWindow{FileID: "code", Lines: []sourceLine{{Number: 1, Text: "core"}, {Number: 2, Text: ""}, {Number: 3, Text: ""}}}
	clipped := clippedInitialWindow(original, 4)
	if len(clipped.Lines) != 1 || !clipped.Truncated {
		t.Fatal("dropped blank lines were reported as a complete window")
	}
}
