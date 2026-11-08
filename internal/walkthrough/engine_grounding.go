package walkthrough

import "strings"

// groundDraft materializes only missing quotes from exact acquired line spans.
// It never searches for a quote, repairs a line range, truncates a premise or
// changes narrative wording. Mechanical validity still requires ValidateDraft;
// semantic support still requires the independent whole-story reviewer.
func groundDraft(bundle Bundle, draft Draft) (Draft, error) {
	if len(bundle.Files) > 128 || len(draft.Statements) > maxStatements {
		return Draft{}, ErrStoryRejected
	}
	files := make(map[string]EvidenceFile, len(bundle.Files))
	starts := make(map[string][]int, len(bundle.Files))
	for _, file := range bundle.Files {
		if _, exists := files[file.ID]; exists {
			return Draft{}, ErrStoryRejected
		}
		files[file.ID] = file
		starts[file.ID] = sourceLineStarts(file.Content)
	}
	draft.Statements = append([]Statement(nil), draft.Statements...)
	for i := range draft.Statements {
		if len(draft.Statements[i].Sources) > maxSources {
			return Draft{}, ErrStoryRejected
		}
		draft.Statements[i].Sources = append([]Citation(nil), draft.Statements[i].Sources...)
		for j := range draft.Statements[i].Sources {
			citation := &draft.Statements[i].Sources[j]
			file, found := files[citation.FileID]
			lineStarts := starts[citation.FileID]
			if !found || citation.StartLine < 1 || citation.EndLine < citation.StartLine || citation.EndLine-citation.StartLine >= maxSpanLines || citation.EndLine > len(lineStarts) {
				return Draft{}, ErrStoryRejected
			}
			if citation.Quote != "" {
				if !validCitation(file.Content, *citation, lineStarts) {
					return Draft{}, ErrStoryRejected
				}
				continue
			}
			// A clipped final line is not a complete original source line.
			if file.Truncated && citation.EndLine == len(lineStarts) && !strings.HasSuffix(file.Content, "\n") {
				return Draft{}, ErrStoryRejected
			}
			end := len(file.Content)
			if citation.EndLine < len(lineStarts) {
				end = lineStarts[citation.EndLine]
			}
			span := file.Content[lineStarts[citation.StartLine-1]:end]
			if len(span) > maxQuoteChars || !validText(span, maxQuoteChars, false) {
				return Draft{}, ErrStoryRejected
			}
			citation.Quote = span
		}
	}
	return draft, nil
}
