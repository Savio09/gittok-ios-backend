package walkthrough

// writerDraft is the model-facing selector contract. The writer never supplies
// internal citation end lines or quote bytes. Trusted conversion and grounding
// precede the unchanged evidence validator and independent semantic reviewer.
type writerDraft struct {
	Statements []writerStatement `json:"statements"`
	Scenes     []Scene           `json:"scenes"`
}

type writerStatement struct {
	ID      string           `json:"id"`
	Kind    string           `json:"kind"`
	Text    string           `json:"text"`
	Sources []writerCitation `json:"sources"`
	Limit   string           `json:"limit"`
}

type writerCitation struct {
	FileID    string `json:"file_id"`
	StartLine int    `json:"start_line"`
	LineCount int    `json:"line_count"`
}

func (writer writerDraft) toDraft() (Draft, error) {
	if len(writer.Statements) > maxStatements || len(writer.Scenes) > maxScenes {
		return Draft{}, ErrStoryRejected
	}
	draft := Draft{Scenes: append([]Scene(nil), writer.Scenes...)}
	for _, statement := range writer.Statements {
		if len(statement.Sources) > maxSources {
			return Draft{}, ErrStoryRejected
		}
		s := Statement{ID: statement.ID, Kind: statement.Kind, Text: statement.Text, Limit: statement.Limit}
		for _, source := range statement.Sources {
			if source.StartLine < 1 || source.LineCount < 1 || source.LineCount > 10 || source.StartLine > int(^uint(0)>>1)-(source.LineCount-1) {
				return Draft{}, ErrStoryRejected
			}
			s.Sources = append(s.Sources, Citation{FileID: source.FileID, StartLine: source.StartLine, EndLine: source.StartLine + (source.LineCount - 1)})
		}
		draft.Statements = append(draft.Statements, s)
	}
	return draft, nil
}
