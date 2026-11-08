package walkthrough

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

const sourceLimit = "Source interpretation only; execution was not observed."
const reviewedExplanation = "Source-based interpretation checked against the cited passages."

// Accept publishes only referenced statements with valid source spans and an
// independent positive review. Callers must separately preserve acquisition
// outcome, generation fencing and current public repository visibility.
func Accept(bundle Bundle, draft Draft, review Review) (Accepted, error) {
	if err := ValidateDraft(bundle, draft); err != nil {
		return Accepted{}, err
	}
	if !review.Accepted || !validText(review.Reason, 1600, false) || len(review.Decisions) > maxStatements {
		return Accepted{}, errors.New("walkthrough: whole-story review required")
	}
	statements := make(map[string]Statement, len(draft.Statements))
	for _, statement := range draft.Statements {
		statements[statement.ID] = statement
	}
	decisions := make(map[string]Decision, len(review.Decisions))
	for _, decision := range review.Decisions {
		_, known := statements[decision.StatementID]
		if !known || !validText(decision.Reason, 1600, false) {
			return Accepted{}, errors.New("walkthrough: invalid review decision")
		}
		if _, duplicate := decisions[decision.StatementID]; duplicate {
			return Accepted{}, errors.New("walkthrough: duplicate review decision")
		}
		decisions[decision.StatementID] = decision
	}
	files := make(map[string]EvidenceFile, len(bundle.Files))
	for _, file := range bundle.Files {
		files[file.ID] = file
	}
	sections := make([]apimodel.Section, 0, len(draft.Scenes))
	prose := 0
	for index, scene := range draft.Scenes {
		section := sceneSection(scene.Role)
		body := make([]string, 0, len(scene.StatementIDs))
		for _, id := range scene.StatementIDs {
			decision, found := decisions[id]
			if !found || !decision.Accepted {
				return Accepted{}, errors.New("walkthrough: referenced statement not accepted")
			}
			statement := statements[id]
			claim := renderClaim(bundle.Identity, statement, files)
			prose += utf8.RuneCountInString(statement.Text)*2 + utf8.RuneCountInString(claim.Limit) + utf8.RuneCountInString(claim.Explanation)
			if prose > maxProseChars {
				return Accepted{}, errors.New("walkthrough: rendered prose limit exceeded")
			}
			section.Claims = append(section.Claims, claim)
			body = append(body, statement.Text)
		}
		section.Body = strings.Join(body, "\n\n")
		section.ID = contentID("scene", struct {
			Identity Identity
			Position int
			Section  apimodel.Section
		}{bundle.Identity, index, section})
		sections = append(sections, section)
	}
	id := contentID("story", struct {
		Identity Identity
		Sections []apimodel.Section
	}{bundle.Identity, sections})
	return Accepted{id: id, identity: bundle.Identity, sections: sections}, nil
}

func renderClaim(identity Identity, statement Statement, files map[string]EvidenceFile) apimodel.Claim {
	sources := make([]apimodel.Source, 0, len(statement.Sources))
	quotes := make([]string, 0, len(statement.Sources))
	components := make([]string, 0, len(statement.Sources))
	seenComponents := make(map[string]bool)
	truncated := false
	for _, citation := range statement.Sources {
		file := files[citation.FileID]
		truncated = truncated || file.Truncated
		sources = append(sources, renderSource(identity, file.Path, citation))
		quotes = append(quotes, citation.Quote)
		if file.Component != "" && !seenComponents[file.Component] {
			components = append(components, file.Component)
			seenComponents[file.Component] = true
		}
	}
	limit := sourceLimit
	if statement.Limit != "" {
		limit = statement.Limit + " " + limit
	}
	if truncated {
		limit += " Cited evidence includes a truncated file."
	}
	if len(components) != 0 {
		limit += " Source components: " + strings.Join(components, ", ") + "."
	}
	claim := apimodel.Claim{
		Label:  statementLabel(statement.Kind),
		Source: sources[0], AdditionalSources: sources[1:],
		Excerpt:     strings.Join(quotes, "\n\n"),
		Explanation: reviewedExplanation, Supports: statement.Text, Limit: limit,
		EvidenceStatus: apimodel.EvidenceInferred, Confidence: apimodel.Uncertain,
	}
	claim.ID = contentID("statement", struct {
		Identity Identity
		Kind     string
		Claim    apimodel.Claim
	}{identity, statement.Kind, claim})
	return claim
}

func renderSource(identity Identity, filePath string, citation Citation) apimodel.Source {
	segments := strings.Split(filePath, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	start, end := strconv.Itoa(citation.StartLine), strconv.Itoa(citation.EndLine)
	return apimodel.Source{
		Kind: apimodel.SourceFile, Ref: filePath, Lines: start + "-" + end,
		URL: "https://github.com/" + url.PathEscape(identity.Owner) + "/" + url.PathEscape(identity.Name) + "/blob/" + identity.Commit + "/" + strings.Join(segments, "/") + "#L" + start + "-L" + end,
	}
}

func sceneSection(role string) apimodel.Section {
	section := apimodel.Section{EvidenceMode: apimodel.NotExecuted}
	switch role {
	case "purpose":
		section.Kind, section.Title = apimodel.SectionIdea, "What it does"
	case "workflow":
		section.Kind, section.Title = apimodel.SectionHow, "How it works"
	case "design":
		section.Kind, section.Title = apimodel.SectionStack, "A closer look"
	case "handoff":
		section.Kind, section.Title = apimodel.SectionSetup, "Where to look next"
	}
	return section
}

func statementLabel(kind string) string {
	switch kind {
	case "purpose":
		return "Purpose"
	case "action":
		return "Input"
	case "mechanism":
		return "Mechanism"
	case "result":
		return "Result"
	case "design":
		return "Design"
	case "handoff":
		return "Next step"
	}
	return ""
}

func contentID(kind string, value any) string {
	// These values contain only JSON-safe structs, strings and bounded integers.
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return kind + "-v1-" + hex.EncodeToString(sum[:])
}
