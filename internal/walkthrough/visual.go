package walkthrough

import (
	"errors"
	"strings"
	"unicode"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Production generation requires a complete film. Legacy Accept deliberately
// keeps its earlier non-visual acceptance contract for stored/source previews.
func validateGeneratedDraft(bundle Bundle, draft Draft) error {
	if err := ValidateDraft(bundle, draft); err != nil {
		return err
	}
	statements := make(map[string]Statement, len(draft.Statements))
	for _, statement := range draft.Statements {
		statements[statement.ID] = statement
	}
	files := make(map[string]EvidenceFile, len(bundle.Files))
	for _, file := range bundle.Files {
		files[file.ID] = file
	}
	seenClaims := make(map[string]bool)
	handoff := false
	for _, scene := range draft.Scenes {
		for _, id := range scene.StatementIDs {
			statement := statements[id]
			if statement.Visual == nil {
				return errors.New("walkthrough: generated visual required")
			}
			// Check publication identity, not writer IDs: distinct draft statements
			// with identical accepted content would collide in the native film too.
			claimID := renderClaim(bundle.Identity, statement, files).ID
			if seenClaims[claimID] {
				return errors.New("walkthrough: duplicate visual claim")
			}
			seenClaims[claimID] = true
			handoff = handoff || scene.Role == "handoff" && statement.Kind == "handoff"
		}
	}
	if !handoff {
		return errors.New("walkthrough: generated handoff required")
	}
	return nil
}

func copyVisual(visual *apimodel.StoryVisual) *apimodel.StoryVisual {
	if visual == nil {
		return nil
	}
	copy := *visual
	copy.Lines = append([]string{}, visual.Lines...)
	return &copy
}

func visualText(s string, maximum int, optional bool) bool {
	if !validText(s, maximum, optional) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// Source visuals use complete original lines from the first actual citation,
// not a model's quote substring. Source tabs/CR and long paths are preserved.
func sourceVisualLines(file EvidenceFile, citation Citation) ([]string, bool) {
	starts := sourceLineStarts(file.Content)
	last := min(citation.EndLine, citation.StartLine+5)
	if citation.StartLine < 1 || last < citation.StartLine || last > len(starts) {
		return nil, false
	}
	if file.Truncated && last == len(starts) && !strings.HasSuffix(file.Content, "\n") {
		return nil, false
	}
	end := len(file.Content)
	if last < len(starts) {
		end = starts[last]
	}
	span := file.Content[starts[citation.StartLine-1]:end]
	if len(span) > maxQuoteChars {
		return nil, false
	}
	return strings.Split(strings.TrimSuffix(span, "\n"), "\n"), true
}

func validVisual(statement Statement, files map[string]EvidenceFile) bool {
	v := statement.Visual
	if v == nil {
		return true
	} // Legacy accepted/source previews remain readable.
	if !visualText(v.Cue, 40, false) || !strings.Contains(statement.Text, v.Cue) || v.Focus < 0 {
		return false
	}
	if v.Kind == "source" {
		if v.Title != "" || len(v.Lines) != 0 || len(statement.Sources) == 0 {
			return false
		}
		lines, ok := sourceVisualLines(files[statement.Sources[0].FileID], statement.Sources[0])
		// The native consumer verifies source visuals against the claim excerpt.
		// A legacy substring quote may be valid evidence while omitting original
		// indentation or other displayed lines; it cannot back a source visual.
		return ok && v.Focus <= len(lines) && strings.HasPrefix(statement.Sources[0].Quote, strings.Join(lines, "\n"))
	}
	if !visualText(v.Title, 56, false) || v.Focus > len(v.Lines) {
		return false
	}
	minimum, maximum := 1, 4
	switch v.Kind {
	case "concept":
		maximum = 3
	case "terminal":
	case "flow":
		minimum = 2
	case "comparison":
		minimum, maximum = 2, 2
	default:
		return false
	}
	if len(v.Lines) < minimum || len(v.Lines) > maximum {
		return false
	}
	for _, line := range v.Lines {
		if !visualText(line, 80, false) {
			return false
		}
	}
	return true
}

func renderVisual(statement Statement, files map[string]EvidenceFile) *apimodel.StoryVisual {
	v := copyVisual(statement.Visual)
	if v != nil && v.Kind == "source" {
		citation := statement.Sources[0]
		v.Title = files[citation.FileID].Path
		v.Lines, _ = sourceVisualLines(files[citation.FileID], citation)
	}
	return v
}

func visualProse(visual *apimodel.StoryVisual) int {
	if visual == nil {
		return 0
	}
	// Count every additional rendered word, including the literal cue.
	return len([]rune(visual.Title + visual.Cue + strings.Join(visual.Lines, "")))
}
