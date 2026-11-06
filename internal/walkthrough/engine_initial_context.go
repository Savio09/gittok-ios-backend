package walkthrough

import (
	"path"
	"strings"
	"unicode/utf8"
)

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
