package walkthrough

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Savio09/gittok-ios-backend/internal/generation"
	"net/http"
	"strings"
	"testing"
)

func TestFreshComposerPreservesDeepRequestedWindowWithoutResearchTranscript(t *testing.T) {
	e, err := NewEngine(coldGenerationPricedConfig(), engineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	bundle := Bundle{Identity: Identity{RepoID: 1, Owner: "owner", Name: "repo", Commit: strings.Repeat("a", 40)}}
	request, _ := initialContextRequest(bundle, nil, []sourceWindow{{FileID: "root", Path: "main.go", Lines: []sourceLine{{Number: 1, Text: "package main"}}}})
	window := sourceWindow{FileID: "deep", Path: "src/handler.go", Lines: []sourceLine{{Number: 1400, Text: "func handle(input Input) Result {"}, {Number: 1401, Text: " return process(input)"}, {Number: 1402, Text: "}"}}}
	encoded, _ := json.Marshal(window)
	output, _ := json.Marshal(map[string]any{"type": "function_call_output", "call_id": "read-deep", "output": string(encoded)})
	reasoning := json.RawMessage(`{"type":"reasoning","encrypted_content":"do-not-copy-research"}`)
	request.Input = append(request.Input, reasoning, output)
	final, forced := e.writerTurn(request, &engineBudget{}, true)
	if !forced || len(final.Input) != 1 {
		t.Fatal("dedicated composer retained research messages")
	}
	if strings.Contains(string(final.Input[0]), "do-not-copy-research") {
		t.Fatal("composer retained encrypted research context")
	}
	var message struct {
		Content string `json:"content"`
	}
	_ = json.Unmarshal(final.Input[0], &message)
	var packet composerContext
	if err := json.Unmarshal([]byte(message.Content), &packet); err != nil {
		t.Fatal(err)
	}
	if len(packet.Requested) != 1 || engineJSON(t, packet.Requested[0]) != engineJSON(t, window) {
		t.Fatal("fresh composition lost or changed the requested deep source lines")
	}
	if _, err := e.prepareRequest(&engineBudget{}, final, false); err != nil {
		t.Fatal(err)
	}
}

func TestComposerHandlesEmptyInputWithoutPanicking(t *testing.T) {
	e := &Engine{cfg: coldGenerationPricedConfig()}
	_ = e.composerRequest(responseRequest{}, &engineBudget{})
}

func TestBoundedToolWindowRetainsCompleteLinesAndHonestTruncation(t *testing.T) {
	original := sourceWindow{FileID: "code", Path: "src/code.go"}
	for i := 1; i <= 80; i++ {
		original.Lines = append(original.Lines, sourceLine{Number: i, Text: strings.Repeat("\t界", 25)})
	}
	clipped, ok := boundedToolResult(original).(sourceWindow)
	if !ok || !clipped.Truncated || len(clipped.Lines) == 0 || len(clipped.Lines) >= len(original.Lines) {
		t.Fatal("funded window not honestly clipped")
	}
	encoded, _ := json.Marshal(clipped)
	if len(encoded) > maxToolResultBytes {
		t.Fatal("tool output exceeded funded serialized bound")
	}
	for i, line := range clipped.Lines {
		if line != original.Lines[i] {
			t.Fatal("source window rewrote or split original lines")
		}
	}
	if len(original.Lines) != 80 {
		t.Fatal("tool fitting mutated its source")
	}
	oversized := sourceWindow{FileID: "code", Path: "code.go", MatchedLine: 99, Lines: []sourceLine{{Number: 99, Text: strings.Repeat("x", maxToolResultBytes)}}}
	if _, ok := boundedToolResult(oversized).(sourceWindow); ok {
		t.Fatal("unfunded matching line masqueraded as a successful search")
	}
}

func TestFundedFreshFinalSurvivesBoundedWindowAndLargeResearchContinuation(t *testing.T) {
	e, err := NewEngine(coldGenerationPricedConfig(), engineLedger(t))
	if err != nil {
		t.Fatal(err)
	}
	request, _ := initialContextRequest(Bundle{Identity: Identity{RepoID: 1, Owner: "owner", Name: "repo", Commit: strings.Repeat("a", 40)}}, nil, nil)
	window := sourceWindow{FileID: "deep", Path: "src/product.go"}
	for i := 1000; i < 1080; i++ {
		window.Lines = append(window.Lines, sourceLine{Number: i, Text: strings.Repeat("x", 110)})
	}
	window = boundedToolResult(window).(sourceWindow)
	encoded, _ := json.Marshal(window)
	if len(encoded) < maxToolResultBytes-300 {
		t.Fatal("test did not exercise a near-cap tool result")
	}
	output, _ := json.Marshal(map[string]any{"type": "function_call_output", "call_id": "deep-read", "output": string(encoded)})
	reasoning, _ := json.Marshal(map[string]any{"type": "reasoning", "encrypted_content": strings.Repeat("r", 30<<10)})
	request.Input = append(request.Input, reasoning, output)
	budget := engineBudget{calls: 2, input: 30_000, output: 600, spent: 50_000}
	final, forced := e.writerTurn(request, &budget, false)
	if !forced {
		t.Fatal("unaffordable research transcript should end research")
	}
	prepared, err := e.prepareRequest(&budget, final, false)
	if err != nil || prepared.maximum > e.allowance(&budget, false).Micros {
		t.Fatal("bounded research consumed the exact fresh-final allowance", err)
	}
	var msg struct {
		Content string `json:"content"`
	}
	_ = json.Unmarshal(final.Input[0], &msg)
	var packet composerContext
	_ = json.Unmarshal([]byte(msg.Content), &packet)
	if len(packet.Requested) != 1 || engineJSON(t, packet.Requested[0]) != engineJSON(t, window) {
		t.Fatal("funding dropped mandatory acquired lines")
	}
}

func TestExhaustedWindowIsDistinguishableWithoutPayingProvider(t *testing.T) {
	ledger := engineLedger(t)
	if _, err := ledger.Reserve("prior-unknown", "hold", 999_999); err != nil {
		t.Fatal(err)
	}
	calls := 0
	engine, err := NewEngine(engineConfig(), ledger, WithHTTPClient(&http.Client{Transport: engineRoundTrip(func(*http.Request) (*http.Response, error) { calls++; return nil, ErrProviderUnavailable })}))
	if err != nil {
		t.Fatal(err)
	}
	message, _ := inputMessage(map[string]string{"source": "bounded fixture"})
	_, err = engine.respond(context.Background(), "unfunded-cold", &engineBudget{}, responseRequest{Input: []json.RawMessage{message}, MaxOutputTokens: 1024}, true)
	if !errors.Is(err, generation.ErrBudgetExceeded) || !errors.Is(err, ErrProviderUnavailable) || calls != 0 {
		t.Fatalf("spend limit hidden as generic failure or provider called: %v calls=%d", err, calls)
	}
}

func TestRevisionPairPreservesBudgetFailureAndReleasesOnlyUnusedReservation(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		prior, first, second int64
	}{{"first", 950_000, 60_000, 60_000}, {"second", 700_000, 200_000, 200_000}} {
		t.Run(tc.name, func(t *testing.T) {
			ledger := engineLedger(t)
			if _, err := ledger.Reserve("prior-unknown", "hold", tc.prior); err != nil {
				t.Fatal(err)
			}
			e := &Engine{ledger: ledger}
			_, _, err := e.reserveRevisionPair("refused-pair", 0, tc.first, tc.second)
			if !errors.Is(err, generation.ErrBudgetExceeded) || !errors.Is(err, ErrProviderUnavailable) {
				t.Fatal("pair erased the spending limit", err)
			}
			if _, err = ledger.Reserve("remaining-proof", "hold", 1_000_000-tc.prior); err != nil {
				t.Fatal("unused first reservation remained charged", err)
			}
		})
	}
}
