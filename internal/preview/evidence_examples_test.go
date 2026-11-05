package preview

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// These fixtures are actual outputs from the production builders, not prose
// examples. Refresh only after auditing changed statements and source scopes.
func TestEvidenceContractExamples(t *testing.T) {
	partial := metadataInput()
	partial.Coverage.ContentsIncomplete = true
	partial.Inspection.Scope.Truncated = true
	repo := metadataInput().Repo
	repo.HTMLURL = "https://github.com/" + repo.FullName
	// Feed ages use the current clock; omit the optional push time so the
	// maintained example is stable. Its truthful label is tested separately.
	repo.PushedAt = time.Time{}
	examples := map[string]any{
		"preview.inferred.json":               Build(corpusInput(t, "cli__cli.json")),
		"pocketbase__pocketbase.preview.json": Build(corpusInput(t, "pocketbase__pocketbase.json")),
		"preview.unknown.json":                Build(inputForTest()),
		"preview.partial.json":                Build(partial),
		"feed.metadata.json":                  apimodel.Feed{Items: []apimodel.RepoPreview{FromMetadata(repo)}},
	}
	for name, value := range examples {
		t.Run(name, func(t *testing.T) {
			b, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			b = append(b, '\n')
			path := filepath.Join("testdata", "g18", name)
			if os.Getenv("GITTOK_UPDATE_G18_EXAMPLES") == "1" {
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, b, 0644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(b, want) {
				t.Fatal("G18 contract example changed; audit before updating")
			}
		})
	}
}
