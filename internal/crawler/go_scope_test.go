package crawler

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestNoServerPremiseRequiresPackageListingCoverage(t *testing.T) {
	for _, tc := range []struct {
		name  string
		scope TreeScope
		want  VerdictStatus
	}{
		{"truncated package", TreeScope{Truncated: true}, VerdictInconclusive},
		{"complete package", TreeScope{}, VerdictResolved},
		{"depth cap covers package files", TreeScope{DepthLimited: true, MaxDepth: 3}, VerdictResolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insp := InspectionFromSources("Go", []string{"go.mod", "cmd/service/main.go"}, map[string]string{
				"go.mod": "module example.com/service\n", "cmd/service/main.go": "package main\nfunc main(){}\n",
			})
			insp.Scope = tc.scope
			got := Classify(insp)
			if got.Status != tc.want || (tc.want == VerdictResolved && got.Kind != apimodel.KindCLI) {
				t.Fatalf("got %s/%s, want status %s: %v", got.Kind, got.Status, tc.want, got.Evidence())
			}
			if tc.want == VerdictInconclusive && !limitationsContain(got.Limitations, "cmd/service") {
				t.Errorf("missing package scope limitation: %v", got.Limitations)
			}
		})
	}
}

func TestNoServerWitnessCannotOmitPackageSiblings(t *testing.T) {
	insp := InspectionFromSources("Go", []string{"go.mod", "cmd/tool/main.go", "cmd/tool/helper.go"}, map[string]string{
		"go.mod": "module example.com/tool\n", "cmd/tool/main.go": "package main\nfunc main(){}\n",
		"cmd/tool/helper.go": "package main\nfunc helper(){}\n",
	})
	full := Witness{Kind: WitnessScope, Paths: []string{"go.mod", "cmd/tool/main.go", "cmd/tool/helper.go"}, Construct: constructGoNoServer}
	if err := validateWitness(indexInspection(insp), full); err != nil {
		t.Fatalf("complete scope rejected: %v", err)
	}
	omitted := full
	omitted.Paths = []string{"go.mod", "cmd/tool/main.go"}
	if err := validateWitness(indexInspection(insp), omitted); err == nil || !strings.Contains(err.Error(), "helper.go") {
		t.Errorf("omitted sibling must be named in a rejection, got %v", err)
	}
	noPackage := full
	noPackage.Paths = []string{"go.mod"}
	if err := validateWitness(indexInspection(insp), noPackage); err == nil {
		t.Error("accepted no-server premise without any package source")
	}
	insp.Scope.Truncated = true
	if err := validateWitness(indexInspection(insp), full); err == nil {
		t.Error("accepted full-package premise with a truncated listing")
	}
}

func TestNoServerWitnessNeedsACommandPackage(t *testing.T) {
	library := InspectionFromSources("Go", []string{"go.mod", "lib.go"}, map[string]string{
		"go.mod": "module example.com/library\n", "lib.go": "package library\n",
	})
	w := Witness{Kind: WitnessScope, Paths: []string{"go.mod", "lib.go"}, Construct: constructGoNoServer}
	if err := validateWitness(indexInspection(library), w); err == nil {
		t.Error("accepted an importable library as a command-package witness")
	}
	for _, clause := range []string{"package library\n", "package helper\n"} {
		insp := InspectionFromSources("Go", []string{"go.mod", "main.go", "helper.go"}, map[string]string{
			"go.mod": "module example.com/tool\n", "main.go": "package main\nfunc main(){}\n", "helper.go": clause,
		})
		for _, basis := range [][]string{{"go.mod", "helper.go"}, {"go.mod", "main.go", "helper.go"}} {
			w := Witness{Kind: WitnessScope, Paths: basis, Construct: constructGoNoServer}
			if err := validateWitness(indexInspection(insp), w); err == nil {
				t.Errorf("accepted a non-command or mixed-package basis: %v", basis)
			}
		}
	}
}
