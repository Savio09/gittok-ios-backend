package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestWorkspaceRootParticipatesInProductResolution(t *testing.T) {
	for _, tc := range []struct {
		name, root, member string
		want               apimodel.RepoKind
		status             VerdictStatus
	}{
		{"root frontend beside a library", `"dependencies":{"react":"18"}`, `{"exports":"./index.js"}`, apimodel.KindFrontend, VerdictResolved},
		{"root backend beside a library", `"dependencies":{"express":"5"}`, `{"exports":"./index.js"}`, apimodel.KindBackend, VerdictResolved},
		{"root command beside a library", `"bin":"./cli.js"`, `{"exports":"./index.js"}`, apimodel.KindCLI, VerdictResolved},
		{"competing root and member artifacts", `"dependencies":{"react":"18"}`, `{"bin":"./cli.js"}`, apimodel.KindOther, VerdictInconclusive},
		{"artifact-free aggregator", `"name":"workspace"`, `{"dependencies":{"react":"18"}}`, apimodel.KindFrontend, VerdictResolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insp := InspectionFromSources("TypeScript",
				[]string{"package.json", "index.html", "packages/member/package.json", "packages/member/index.html"},
				map[string]string{
					"package.json":                 `{"private":true,"workspaces":["packages/*"],` + tc.root + `}`,
					"packages/member/package.json": tc.member,
				})
			// A library member has no application shell; keep its fixture independent.
			if tc.member == `{"exports":"./index.js"}` {
				insp.Paths = insp.Paths[:3]
			}
			got := Classify(insp)
			if got.Kind != tc.want || got.Status != tc.status {
				t.Fatalf("got %s/%s, want %s/%s: %v", got.Kind, got.Status, tc.want, tc.status, got.Evidence())
			}
			if tc.status == VerdictResolved && tc.name != "artifact-free aggregator" {
				for _, o := range got.Observations {
					if o.Component != "" {
						t.Errorf("root product replaced by member evidence: %+v", o)
					}
				}
			}
		})
	}
}
