package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestLibraryNeedsKnownConventionalCommandClauses(t *testing.T) {
	for _, tc := range []struct {
		name, clause string
		clipped      bool
		want         VerdictStatus
	}{
		{"clipped unknown clause", "package ", true, VerdictInconclusive},
		{"malformed unknown clause", "package ???\n", false, VerdictInconclusive},
		{"known non-main clause in clipped source", "package cmd\n", true, VerdictResolved},
		{"complete non-main source", "package cmd\nfunc Helper(){}\n", false, VerdictResolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insp := InspectionFromSources("Go", []string{"go.mod", "lib.go", "cmd/service/entry.go"}, map[string]string{
				"go.mod": "module example.com/core\n", "lib.go": "package core\n", "cmd/service/entry.go": tc.clause,
			})
			for n := range insp.Files {
				if insp.Files[n].Path == "cmd/service/entry.go" && tc.clipped {
					insp.Files[n].Truncated, insp.Files[n].Size = true, 1000
				}
			}
			got := Classify(insp)
			if got.Status != tc.want || (tc.want == VerdictResolved && got.Kind != apimodel.KindLibrary) {
				t.Fatalf("got %s/%s, want status %s: %v", got.Kind, got.Status, tc.want, got.Evidence())
			}
			if tc.want == VerdictInconclusive && !limitationsContain(got.Limitations, "cmd/service/entry.go") {
				t.Errorf("missing unknown-clause limitation: %v", got.Limitations)
			}
		})
	}
}
