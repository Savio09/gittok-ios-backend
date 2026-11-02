package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestNPMBinNeedsMeaningfulExecutableEntries(t *testing.T) {
	for _, tc := range []struct {
		name, bin string
		want      apimodel.RepoKind
		status    VerdictStatus
	}{
		{"absent", "", apimodel.KindLibrary, VerdictResolved},
		{"null", "null", apimodel.KindLibrary, VerdictResolved},
		{"empty object", `{}`, apimodel.KindLibrary, VerdictResolved},
		{"empty string", `""`, apimodel.KindLibrary, VerdictResolved},
		{"string executable", `"./cli.js"`, apimodel.KindCLI, VerdictResolved},
		{"command mapping", `{"tool":"./cli.js"}`, apimodel.KindCLI, VerdictResolved},
		{"array", `[]`, apimodel.KindOther, VerdictInconclusive},
		{"number", `42`, apimodel.KindOther, VerdictInconclusive},
		{"boolean", `false`, apimodel.KindOther, VerdictInconclusive},
		{"empty command", `{"":"./cli.js"}`, apimodel.KindOther, VerdictInconclusive},
		{"empty target", `{"tool":""}`, apimodel.KindOther, VerdictInconclusive},
		{"wrong target type", `{"tool":false}`, apimodel.KindOther, VerdictInconclusive},
		{"whitespace target", `"   "`, apimodel.KindOther, VerdictInconclusive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := `{"name":"core","exports":"./index.js"`
			if tc.bin != "" {
				manifest += `,"bin":` + tc.bin
			}
			manifest += `}`
			got := Classify(InspectionFromSources("TypeScript", []string{"package.json", "index.js"}, map[string]string{"package.json": manifest}))
			if got.Kind != tc.want || got.Status != tc.status {
				t.Fatalf("got %s/%s, want %s/%s: %v", got.Kind, got.Status, tc.want, tc.status, got.Evidence())
			}
			if tc.status == VerdictInconclusive && !limitationsContain(got.Limitations, "bin") {
				t.Errorf("invalid entry needs a named limitation: %v", got.Limitations)
			}
			if tc.want == apimodel.KindLibrary && tc.bin != "" {
				for _, o := range got.Observations {
					if strings.Contains(o.Detail, `declares no "bin" field`) {
						t.Errorf("empty executable declaration relabelled as an absent field: %s", o.Detail)
					}
				}
			}
			if tc.want == apimodel.KindCLI {
				if len(got.Observations) == 0 || got.Observations[0].Witness.Value != "./cli.js" {
					t.Errorf("command declaration lacks its executable target: %+v", got.Observations)
				}
			}
		})
	}
}
