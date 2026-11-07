package walkthrough

import (
	"encoding/json"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func initialContextRequest(bundle Bundle, paths []string, windows []sourceWindow) (responseRequest, error) {
	message, err := inputMessage(map[string]any{"identity": bundle.Identity, "limitations": bundle.Limitations, "listed_paths": paths, "inspected_source_windows": windows})
	if err != nil {
		return responseRequest{}, err
	}
	return responseRequest{Instructions: writerInstructions, Input: []json.RawMessage{message}, Tools: writerTools(), ToolChoice: "required"}, nil
}

// Probe the exact dedicated-final request before any ledger/provider action.
// Only presentation windows change; the pinned acquired Bundle stays intact.
func (e *Engine) fittedInitialRequest(bundle Bundle, paths []string) (responseRequest, error) {
	windows := initialEvidenceWindows(bundle.Files)
	fit := func(projected []sourceWindow) (responseRequest, bool) {
		request, err := initialContextRequest(bundle, paths, projected)
		if err != nil {
			return responseRequest{}, false
		}
		final, _ := e.writerTurn(request, &engineBudget{}, true)
		_, err = e.prepareRequest(&engineBudget{}, final, false)
		return request, err == nil
	}
	if request, ok := fit(windows); ok {
		return request, nil
	}
	if len(windows) == 0 {
		return responseRequest{}, ErrGenerationBudget
	}
	anchor := rankedInitialAnchor(bundle.Files, windows)
	if anchor < 0 {
		anchor = 0
	}
	trim := func(prefix []sourceWindow, window sourceWindow) (sourceWindow, bool) {
		low, high := 1, initialWindowBytes(window)
		var best sourceWindow
		found := false
		for low <= high {
			mid := low + (high-low)/2
			candidate := clippedInitialWindow(window, mid)
			if _, ok := fit(append(append([]sourceWindow(nil), prefix...), candidate)); ok {
				best = candidate
				found = len(candidate.Lines) > 0
				low = mid + 1
			} else {
				high = mid - 1
			}
		}
		return best, found
	}
	core := windows[anchor]
	if _, ok := fit([]sourceWindow{core}); !ok {
		var found bool
		core, found = trim(nil, core)
		if !found {
			return responseRequest{}, ErrGenerationBudget
		}
	}
	selected := []sourceWindow{core}
	for i, window := range windows {
		if i == anchor {
			continue
		}
		candidate := append(append([]sourceWindow(nil), selected...), window)
		if _, ok := fit(candidate); ok {
			selected = candidate
			continue
		}
		// Drop expensive secondary code rather than crowding out a short
		// purpose/usage document. Metadata can retain an honestly clipped prefix.
		if !initialCodeContext(window.Path) {
			if partial, ok := trim(selected, window); ok {
				selected = append(selected, partial)
			}
		}
	}
	if request, ok := fit(selected); ok {
		return request, nil
	}
	return responseRequest{}, ErrGenerationBudget
}

func initialWindowBytes(window sourceWindow) int {
	total := 0
	for _, line := range window.Lines {
		total += len(line.Text)
	}
	return total
}

func rankedInitialAnchor(files []EvidenceFile, windows []sourceWindow) int {
	entries := make([]githubclient.TreeEntry, 0, len(files))
	indexes := map[string]int{}
	for _, file := range files {
		entries = append(entries, githubclient.TreeEntry{Path: file.Path, Type: "blob", Mode: "100644"})
	}
	for i, window := range windows {
		if initialCodeContext(window.Path) {
			indexes[window.Path] = i
		}
	}
	// A one-file selection preserves the crawler's reserved primary entry
	// guarantee, even when a build config has a higher ordinary score. The
	// full ranking then supplies primary non-entry product code if necessary.
	for _, limit := range []int{1, len(entries)} {
		budget := crawler.DefaultSelectionBudget()
		budget.MaxFiles = limit
		budget.MaxEntry = 1
		budget.MaxSource = len(entries)
		budget.MaxPerBase = len(entries)
		for _, selected := range crawler.SelectFiles(entries, budget) {
			if i, ok := indexes[selected.Path]; ok && !strings.Contains(selected.Reason, "supporting/test path") {
				return i
			}
		}
	}
	anchor := -1
	for i, window := range windows {
		if initialCodeContext(window.Path) && (anchor < 0 || initialWindowBytes(window) > initialWindowBytes(windows[anchor])) {
			anchor = i
		}
	}
	return anchor
}

func clippedInitialWindow(window sourceWindow, limit int) sourceWindow {
	result := window
	result.Lines = []sourceLine{}
	remaining := limit
	for _, line := range window.Lines {
		if remaining <= 0 {
			break
		}
		text := line.Text
		if len(text) > remaining {
			text = text[:remaining]
			for len(text) > 0 && !utf8.ValidString(text) {
				text = text[:len(text)-1]
			}
		}
		if text == "" && line.Text != "" {
			break
		}
		result.Lines = append(result.Lines, sourceLine{Number: line.Number, Text: text})
		remaining -= len(text)
		if len(text) < len(line.Text) {
			break
		}
	}
	changed := len(result.Lines) != len(window.Lines)
	for i, line := range result.Lines {
		changed = changed || line != window.Lines[i]
	}
	result.Truncated = window.Truncated || changed
	return result
}

// initialEvidenceWindows projects already acquired source; it never reads files
// or renews the investigation. Give product text room beyond entry imports while
// keeping docs/project metadata short. The complete serialized request still
// passes the cumulative generation budget before any provider reservation.
func initialEvidenceWindows(files []EvidenceFile) []sourceWindow {
	remaining := 64 << 10
	var windows []sourceWindow
	for _, code := range []bool{true, false} {
		for _, file := range files {
			if len(windows) >= 6 || remaining == 0 {
				return windows
			}
			if initialCodeContext(file.Path) != code || file.Content == "" {
				continue
			}
			bytes, lines := 8192, 40
			if code {
				bytes, lines = 24<<10, 800
			}
			bytes = min(bytes, remaining)
			original := file.Content
			if len(file.Content) > bytes {
				file.Content = file.Content[:bytes]
				for len(file.Content) > 0 && !utf8.ValidString(file.Content) {
					file.Content = file.Content[:len(file.Content)-1]
				}
				file.Truncated = true
			}
			if file.Content == "" {
				continue
			}
			starts := sourceLineStarts(file.Content)
			lines = min(lines, len(starts))
			window := boundedEvidenceWindow(file, 1, lines, bytes)
			used := len(file.Content)
			if lines < len(starts) {
				used = starts[lines]
			}
			window.Truncated = window.Truncated || used < len(original)
			remaining -= used
			windows = append(windows, window)
		}
	}
	return windows
}

// Format-level metadata distinction, independent of repository identity or
// project kind. Unfamiliar acquired text formats remain eligible for code room.
func initialCodeContext(p string) bool {
	base := strings.ToLower(path.Base(p))
	for _, prefix := range []string{"readme", "license", "contributing", "changelog"} {
		if strings.HasPrefix(base, prefix) {
			return false
		}
	}
	switch base {
	case "setup.py", "gemfile", "dockerfile", "makefile", "procfile", "package.swift", "build.gradle.kts":
		return false
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".md", ".markdown", ".rst", ".adoc", ".asciidoc", ".txt", ".text", ".json", ".toml", ".yaml", ".yml", ".xml", ".ini", ".cfg", ".conf", ".lock", ".mod", ".sum", ".properties", ".gradle", ".csv", ".tsv":
		return false
	}
	return true
}
