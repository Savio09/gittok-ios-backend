package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFirstWriterSeesAlreadyAcquiredProductCodeBeyondLine309(t *testing.T) {
	inv, g, _, _ := engineInvestigation(t)
	g.files["src/report.rs"] = strings.Repeat("// already acquired introductory code\n", 308) + "fn render_product(input: Rows) -> Report { aggregate(input) }\n" + strings.Repeat("// subsequent code\n", 170)
	calls := 0
	engine, err := NewEngine(engineConfig(), engineLedger(t), WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
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
		var packet struct {
			Windows []sourceWindow `json:"inspected_source_windows"`
		}
		if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, window := range packet.Windows {
			if window.Path == "src/report.rs" {
				for _, line := range window.Lines {
					found = found || line.Number == 309 && strings.Contains(line.Text, "fn render_product")
				}
			}
		}
		if !found {
			t.Error("first writer request omitted retained product implementation after imports/comments")
		}
		return nil, errors.New("stop after inspecting initial request")
	})}))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = engine.Run(context.Background(), "initial-context-one", inv, nil)
	readCount, used := len(g.reads), inv.used
	_, _ = engine.Run(context.Background(), "initial-context-two", inv, nil)
	if calls != 2 || len(g.reads) != readCount || inv.used != used || used < len(g.files["src/report.rs"]) {
		t.Fatalf("context projection renewed seed quotas: calls=%d reads=%d/%d used=%d/%d", calls, len(g.reads), readCount, inv.used, used)
	}
}

func TestInitialContextPrioritizesCodeAndKeepsDocsAndManifestsShort(t *testing.T) {
	files := []EvidenceFile{
		{ID: "doc", Path: "README.md", Content: strings.Repeat("documentation\n", 100)},
		{ID: "manifest", Path: "package.json", Content: strings.Repeat("manifest\n", 100)},
		{ID: "setup", Path: "setup.py", Content: strings.Repeat("project metadata\n", 100)},
		{ID: "code", Path: "main.go", Content: strings.Repeat("// product implementation\n", 479)},
	}
	windows := initialEvidenceWindows(files)
	if len(windows) != 4 || windows[0].FileID != "code" || len(windows[0].Lines) != 479 || windows[0].Truncated {
		t.Fatal("already acquired product code was not prioritized or shown in full")
	}
	for _, window := range windows[1:] {
		if len(window.Lines) != 40 || !window.Truncated {
			t.Fatal("document/manifest window exceeded40 lines or hid clipping")
		}
	}
}

func TestInitialContextEnforcesCumulativeBytesFilesLinesAndUTF8(t *testing.T) {
	var files []EvidenceFile
	for _, path := range []string{"a.go", "b.rs", "c.py", "d.ts", "e.java", "f.swift", "g.unfamiliar"} {
		files = append(files, EvidenceFile{ID: path, Path: path, Content: strings.Repeat("界", 11000)})
	}
	windows := initialEvidenceWindows(files)
	bytes := 0
	for _, window := range windows {
		if len(window.Lines) != 1 || !utf8.ValidString(window.Lines[0].Text) || !window.Truncated || len(window.Lines[0].Text) > 24<<10 {
			t.Fatal("context byte boundary split UTF8 or exceeded per-code cap")
		}
		bytes += len(window.Lines[0].Text)
	}
	if bytes > 64<<10 || len(windows) > 6 || bytes < 63<<10 {
		t.Fatalf("cumulative initial context cap incorrect: bytes=%d files=%d", bytes, len(windows))
	}
	lines := initialEvidenceWindows([]EvidenceFile{{ID: "lines", Path: "core.unfamiliar", Content: strings.Repeat("x\n", 900)}})
	if len(lines) != 1 || len(lines[0].Lines) != 800 || !lines[0].Truncated {
		t.Fatal("initial code line cap or clipping annotation incorrect")
	}
	var small []EvidenceFile
	for _, path := range []string{"a.go", "b.rs", "c.py", "d.ts", "e.java", "f.swift", "g.unfamiliar"} {
		small = append(small, EvidenceFile{ID: path, Path: path, Content: "one\n"})
	}
	if got := len(initialEvidenceWindows(small)); got != 6 {
		t.Fatalf("initial context file cap=%d", got)
	}
}
