package walkthrough

import "encoding/json"

// A dedicated composer receives authoritative source windows, not a research
// transcript or its encrypted reasoning. Requested windows are mandatory; seed
// prefixes may shrink only after research has supplied more precise evidence.
type composerContext struct {
	Identity    Identity       `json:"identity"`
	Limitations []string       `json:"limitations"`
	Paths       []string       `json:"listed_paths"`
	Seed        []sourceWindow `json:"inspected_source_windows"`
	Requested   []sourceWindow `json:"requested_source_windows,omitempty"`
}

func (e *Engine) composerRequest(research responseRequest, budget *engineBudget) responseRequest {
	var packet composerContext
	if len(research.Input) > 0 {
		var message struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(research.Input[0], &message) == nil {
			_ = json.Unmarshal([]byte(message.Content), &packet)
		}
	}
	seen := map[string]bool{}
	for index, raw := range research.Input {
		if index == 0 {
			continue
		}
		var item struct {
			Type   string `json:"type"`
			Output string `json:"output"`
		}
		if json.Unmarshal(raw, &item) != nil || item.Type != "function_call_output" {
			continue
		}
		var window sourceWindow
		if json.Unmarshal([]byte(item.Output), &window) != nil || window.FileID == "" || window.Path == "" || len(window.Lines) == 0 {
			continue
		}
		key, _ := json.Marshal(window)
		if !seen[string(key)] {
			packet.Requested = append(packet.Requested, window)
			seen[string(key)] = true
		}
	}
	makeRequest := func() responseRequest {
		message, err := inputMessage(packet)
		result := research
		if err != nil {
			result.Input = nil
		} else {
			result.Input = []json.RawMessage{message}
		}
		return result
	}
	result := makeRequest()
	// Initial fitting owns the seed allocation. After a tool result, retain every
	// requested line while making optional old prefixes fit the remaining money.
	if len(research.Input) <= 1 {
		return result
	}
	for {
		if _, err := e.prepareRequest(budget, result, false); err == nil {
			return result
		}
		largest := -1
		for i, window := range packet.Seed {
			if len(window.Lines) > 1 && (largest < 0 || initialWindowBytes(window) > initialWindowBytes(packet.Seed[largest])) {
				largest = i
			}
		}
		if largest < 0 {
			return result
		} // Exact preflight refuses an unfunded mandatory packet.
		window := packet.Seed[largest]
		window.Lines = window.Lines[:max(1, len(window.Lines)/2)]
		window.Truncated = true
		packet.Seed[largest] = window
		result = makeRequest()
	}
}

// Keep complete original lines and honest clipping in a funded tool envelope.
// An oversized matching line cannot masquerade as a successful search result.
func boundedToolResult(result any) any {
	window, ok := result.(sourceWindow)
	if !ok {
		return result
	}
	for {
		encoded, err := json.Marshal(window)
		if err == nil && len(encoded) <= maxToolResultBytes {
			return window
		}
		if len(window.Lines) <= 1 {
			if window.MatchedLine > 0 {
				return map[string]any{"error": "matching_line_exceeds_window", "matched_line": window.MatchedLine, "file_id": window.FileID}
			}
			return map[string]string{"error": "source_window_exceeds_serialized_limit"}
		}
		window.Lines = window.Lines[:len(window.Lines)-1]
		window.Truncated = true
	}
}
