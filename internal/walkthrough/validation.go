package walkthrough

import (
	"errors"
	"path"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Savio09/gittok-ios-backend/internal/corpus"
)

const (
	maxScenes          = 12
	maxStatements      = 40
	maxStatementChars  = 800
	maxQuoteChars      = 2400
	maxSpanLines       = 30
	maxProseChars      = 10000
	maxSources         = 12
	maxEvidenceBytes   = 8 << 20
	maxSourcePathBytes = 512
)

var (
	ownerPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	repoPattern   = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)
	commitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
	idPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
)

// ValidateDraft checks bounded shape, source integrity and the minimum story
// structure before incurring semantic review cost. It deliberately cannot prove
// that a quoted source entails a statement or that a relationship is directed
// correctly. Accept requires independent whole-story and statement review.
func ValidateDraft(bundle Bundle, draft Draft) error {
	files, err := validateBundle(bundle)
	if err != nil {
		return err
	}
	if len(draft.Statements) == 0 || len(draft.Statements) > maxStatements || len(draft.Scenes) == 0 || len(draft.Scenes) > maxScenes {
		return errors.New("walkthrough: invalid story size")
	}
	statements := make(map[string]Statement, len(draft.Statements))
	lineStarts := make(map[string][]int)
	prose := 0
	for _, statement := range draft.Statements {
		if !idPattern.MatchString(statement.ID) || !validStatementKind(statement.Kind) || !validText(statement.Text, maxStatementChars, false) || !validText(statement.Limit, maxStatementChars, true) {
			return errors.New("walkthrough: invalid statement")
		}
		if _, duplicate := statements[statement.ID]; duplicate {
			return errors.New("walkthrough: duplicate statement")
		}
		if len(statement.Sources) == 0 || len(statement.Sources) > maxSources {
			return errors.New("walkthrough: invalid citation count")
		}
		for _, citation := range statement.Sources {
			file, found := files[citation.FileID]
			if !found {
				return errors.New("walkthrough: invalid citation")
			}
			starts, indexed := lineStarts[citation.FileID]
			if !indexed {
				starts = sourceLineStarts(file.Content)
				lineStarts[citation.FileID] = starts
			}
			if !validCitation(file.Content, citation, starts) {
				return errors.New("walkthrough: invalid citation")
			}
		}
		statements[statement.ID] = statement
		prose += utf8.RuneCountInString(statement.Text) + utf8.RuneCountInString(statement.Limit)
	}
	if prose > maxProseChars {
		return errors.New("walkthrough: prose limit exceeded")
	}
	sceneIDs := make(map[string]bool, len(draft.Scenes))
	hasPurpose, hasWorkflow := false, false
	publishedProse := 0
	for _, scene := range draft.Scenes {
		if !idPattern.MatchString(scene.ID) || sceneIDs[scene.ID] || !validSceneRole(scene.Role) || len(scene.StatementIDs) == 0 || len(scene.StatementIDs) > maxStatements {
			return errors.New("walkthrough: invalid scene")
		}
		sceneIDs[scene.ID] = true
		seen := make(map[string]bool, len(scene.StatementIDs))
		kinds := make(map[string]bool)
		for _, id := range scene.StatementIDs {
			statement, found := statements[id]
			if !found || seen[id] {
				return errors.New("walkthrough: invalid statement reference")
			}
			seen[id] = true
			kinds[statement.Kind] = true
			hasPurpose = hasPurpose || statement.Kind == "purpose"
			publishedProse += utf8.RuneCountInString(statement.Text) + utf8.RuneCountInString(statement.Limit)
		}
		hasWorkflow = hasWorkflow || (scene.Role == "workflow" && kinds["action"] && kinds["mechanism"] && kinds["result"])
	}
	if publishedProse > maxProseChars {
		return errors.New("walkthrough: prose limit exceeded")
	}
	if !hasPurpose || !hasWorkflow {
		return errors.New("walkthrough: incomplete purpose or workflow")
	}
	return nil
}

func validateBundle(bundle Bundle) (map[string]EvidenceFile, error) {
	identity := bundle.Identity
	if identity.RepoID <= 0 || !ownerPattern.MatchString(identity.Owner) || !repoPattern.MatchString(identity.Name) || identity.Name == "." || identity.Name == ".." || !commitPattern.MatchString(identity.Commit) {
		return nil, errors.New("walkthrough: invalid pinned identity")
	}
	// The path cap covers the maximum accepted analysis.Limits listing.
	if len(bundle.Files) == 0 || len(bundle.Files) > 128 || len(bundle.Paths) > 10000 || len(bundle.Limitations) > 128 {
		return nil, errors.New("walkthrough: invalid evidence size")
	}
	files := make(map[string]EvidenceFile, len(bundle.Files))
	paths := make(map[string]bool, len(bundle.Files))
	bytes := 0
	for _, file := range bundle.Files {
		if len(file.Content) > maxEvidenceBytes-bytes {
			return nil, errors.New("walkthrough: evidence limit exceeded")
		}
		bytes += len(file.Content)
		if !idPattern.MatchString(file.ID) || !safeSourcePath(file.Path) || (file.Component != "" && !safeSourcePath(file.Component)) || !utf8.ValidString(file.Content) || strings.ContainsRune(file.Content, 0) || corpus.Secretish(file.Path, file.Content) {
			return nil, errors.New("walkthrough: invalid inspected file")
		}
		if _, duplicate := files[file.ID]; duplicate || paths[file.Path] {
			return nil, errors.New("walkthrough: duplicate inspected file")
		}
		files[file.ID], paths[file.Path] = file, true
	}
	listed := make(map[string]bool, len(bundle.Paths))
	for _, p := range bundle.Paths {
		// A listed file is not a citation source and may include a credential
		// filename. It still must be a canonical repository-relative path.
		if !regularPath(p) || listed[p] {
			return nil, errors.New("walkthrough: invalid listed path")
		}
		listed[p] = true
	}
	for _, limitation := range bundle.Limitations {
		if !validText(limitation, maxStatementChars, false) {
			return nil, errors.New("walkthrough: invalid evidence limitation")
		}
	}
	return files, nil
}

func sourceLineStarts(content string) []int {
	if content == "" {
		return nil
	}
	starts := make([]int, 1, 1+strings.Count(content, "\n"))
	for i := 0; i < len(content)-1; i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}

func validCitation(content string, citation Citation, starts []int) bool {
	if citation.StartLine < 1 || citation.EndLine < citation.StartLine || citation.EndLine-citation.StartLine >= maxSpanLines || !validText(citation.Quote, maxQuoteChars, false) {
		return false
	}
	if citation.EndLine > len(starts) {
		return false
	}
	end := len(content)
	if citation.EndLine < len(starts) {
		end = starts[citation.EndLine]
	}
	span := content[starts[citation.StartLine-1]:end]
	return strings.Contains(span, citation.Quote)
}

func validText(s string, maxChars int, optional bool) bool {
	if !utf8.ValidString(s) || len(s) > maxChars*utf8.UTFMax || utf8.RuneCountInString(s) > maxChars || (!optional && strings.TrimSpace(s) == "") {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

func regularPath(p string) bool {
	if p == "" || len(p) > maxSourcePathBytes || !utf8.ValidString(p) || path.IsAbs(p) || (len(p) > 1 && p[1] == ':') || path.Clean(p) != p || strings.Contains(p, "\\") {
		return false
	}
	for _, r := range p {
		if unicode.IsControl(r) {
			return false
		}
	}
	for _, segment := range strings.Split(p, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func safeSourcePath(p string) bool {
	if !regularPath(p) {
		return false
	}
	for _, segment := range strings.Split(strings.ToLower(p), "/") {
		switch segment {
		case ".npmrc", ".netrc", ".pypirc", ".git-credentials":
			return false
		}
		if segment == ".git" || segment == ".aws" || segment == ".ssh" || strings.HasPrefix(segment, ".env") || segment == "id_rsa" || segment == "id_dsa" || segment == "id_ecdsa" || segment == "id_ed25519" {
			return false
		}
		base := strings.TrimSuffix(segment, path.Ext(segment))
		if base == "credentials" || base == "credential" || base == "secrets" || base == "secret" || base == "tokens" || base == "token" {
			return false
		}
		switch path.Ext(segment) {
		case ".pem", ".key", ".p12", ".pfx":
			return false
		}
	}
	return true
}

func validStatementKind(kind string) bool {
	switch kind {
	case "purpose", "action", "mechanism", "result", "design", "handoff":
		return true
	}
	return false
}

func validSceneRole(role string) bool {
	switch role {
	case "purpose", "workflow", "design", "handoff":
		return true
	}
	return false
}
