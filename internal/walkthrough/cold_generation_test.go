package walkthrough

import (
	"strings"
	"testing"
)

func coldGenerationPricedConfig() EngineConfig {
	cfg := engineConfig()
	cfg.Model = "gpt-6.1-sol"
	cfg.ReasoningEffort = "low"
	cfg.InputMicrosPerMillion = 2_500_000
	cfg.OutputMicrosPerMillion = 10_000_000
	cfg.MaxCalls = 8
	cfg.MaxInputBytes = 512 << 10
	cfg.MaxOutputTokens = 12_000
	cfg.MaxJobMicros = 500_000
	cfg.MaxRevisions = 1
	return cfg
}

func TestColdGenerationTinyRepositoryCanExploreAtConfiguredPrices(t *testing.T) {
	engine, err := NewEngine(coldGenerationPricedConfig(), engineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	bundle := Bundle{
		Identity: Identity{RepoID: 1, Owner: "owner", Name: "project", Commit: strings.Repeat("a", 40)},
		Files: []EvidenceFile{
			{ID: "doc", Path: "README.md", Content: "Convert incoming rows into a report.\n"},
			{ID: "code", Path: "main.go", Content: "package main\nfunc summarize(rows []Row) Report {\n return Report{Totals: aggregate(rows)}\n}\n"},
		},
	}
	request, err := engine.fittedInitialRequest(bundle, []string{"README.md", "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	first, final := engine.writerTurn(request, &engineBudget{}, false)
	if final {
		t.Fatal("tiny repository was forced to submit before any source exploration at the configured prices")
	}
	readable := false
	for _, tool := range first.Tools {
		if tool.Name == "submit_story" {
			t.Error("research turn exposed story submission")
		}
		readable = readable || tool.Name == "read_file"
	}
	if !readable {
		t.Error("affordable first turn did not expose source reads")
	}
	if _, err := engine.prepareRequest(&engineBudget{}, first, false); err != nil {
		t.Fatalf("first source exploration was not affordable: %v", err)
	}
}

func TestColdGenerationLargeAnchorRetainsPurposeAndAffordableExploration(t *testing.T) {
	engine, err := NewEngine(coldGenerationPricedConfig(), engineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	purpose := "Convert incoming rows into a report."
	bundle := Bundle{
		Identity: Identity{RepoID: 1, Owner: "owner", Name: "project", Commit: strings.Repeat("a", 40)},
		Files: []EvidenceFile{
			{ID: "doc", Path: "README.md", Content: purpose + "\n" + strings.Repeat("Usage and source notes.\n", 60)},
			{ID: "code", Path: "main.go", Content: "package main\n" + strings.Repeat("// product context before handling\n", 700) + "func summarize(rows []Row) Report { return aggregate(rows) }\n"},
		},
	}
	before := engineJSON(t, bundle)
	request, err := engine.fittedInitialRequest(bundle, []string{"README.md", "main.go"})
	if err != nil {
		t.Fatal(err)
	}
	hasPurpose, hasCode := false, false
	for _, window := range initialBudgetWindows(t, request) {
		hasCode = hasCode || window.FileID == "code" && len(window.Lines) > 0
		for _, line := range window.Lines {
			hasPurpose = hasPurpose || window.FileID == "doc" && strings.Contains(line.Text, purpose)
		}
	}
	if !hasPurpose || !hasCode {
		t.Errorf("large source prefix displaced useful initial evidence: purpose=%v code=%v", hasPurpose, hasCode)
	}
	if engineJSON(t, bundle) != before {
		t.Error("presentation fitting changed the acquired source bundle")
	}
	first, final := engine.writerTurn(request, &engineBudget{}, false)
	if final {
		t.Error("large initial context consumed the budget needed for bounded source exploration")
	}
	if _, err := engine.prepareRequest(&engineBudget{}, first, false); err != nil {
		t.Errorf("fitted first writer request exceeds its configured allowance: %v", err)
	}
}
