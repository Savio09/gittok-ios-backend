package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestCargoBinaryEntriesAndListingBounds(t *testing.T) {
	for _, tc := range []struct {
		name, entry, settings string
		scope                 TreeScope
		want                  apimodel.RepoKind
		status                VerdictStatus
	}{
		{"direct binary", "src/bin/tool.rs", "", TreeScope{}, apimodel.KindCLI, VerdictResolved},
		{"multi-file binary", "src/bin/tool/main.rs", "", TreeScope{}, apimodel.KindCLI, VerdictResolved},
		{"helper is not an entry", "src/bin/tool/support.rs", "", TreeScope{}, apimodel.KindLibrary, VerdictResolved},
		{"deep helper is not an entry", "src/bin/tool/modules/helper.rs", "", TreeScope{}, apimodel.KindLibrary, VerdictResolved},
		{"depth cap hides main", "src/bin/tool", "", TreeScope{DepthLimited: true, MaxDepth: 3}, apimodel.KindOther, VerdictInconclusive},
		{"depth cap covers entries", "src/bin/tool", "", TreeScope{DepthLimited: true, MaxDepth: 4}, apimodel.KindLibrary, VerdictResolved},
		{"discovery disabled", "src/bin/tool/main.rs", "autobins=false\n", TreeScope{}, apimodel.KindLibrary, VerdictResolved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			insp := InspectionFromSources("Rust", []string{"Cargo.toml", "src/lib.rs", tc.entry}, map[string]string{
				"Cargo.toml": "[package]\nname='core'\nversion='0.1.0'\nedition='2021'\n" + tc.settings,
			})
			insp.Scope = tc.scope
			got := Classify(insp)
			if got.Kind != tc.want || got.Status != tc.status {
				t.Fatalf("got %s/%s, want %s/%s: %v", got.Kind, got.Status, tc.want, tc.status, got.Evidence())
			}
		})
	}
}

func TestImplicitCargoWitnessNeedsItsOwningCrate(t *testing.T) {
	insp := InspectionFromSources("Rust", []string{"src/main.rs", "a/Cargo.toml", "a/src/main.rs", "a/src/bin/tool/support.rs", "b/Cargo.toml", "b/src/main.rs", "virtual/Cargo.toml", "virtual/src/main.rs"}, map[string]string{
		"a/Cargo.toml":       "[package]\nname='a'\nedition='2021'\n",
		"b/Cargo.toml":       "[package]\nname='b'\nautobins=false\n",
		"virtual/Cargo.toml": "[workspace]\nmembers=[]\n",
	})
	for _, tc := range []struct {
		name, manifest, entry string
		valid                 bool
	}{
		{"own binary", "a/Cargo.toml", "a/src/main.rs", true},
		{"other crate's binary", "a/Cargo.toml", "b/src/main.rs", false},
		{"root binary is not a nested crate's entry", "a/Cargo.toml", "src/main.rs", false},
		{"missing owner", "", "a/src/main.rs", false},
		{"helper only", "a/Cargo.toml", "a/src/bin/tool/support.rs", false},
		{"discovery disabled", "b/Cargo.toml", "b/src/main.rs", false},
		{"virtual workspace has no package target", "virtual/Cargo.toml", "virtual/src/main.rs", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := Witness{Kind: WitnessListing, Path: tc.manifest, Paths: []string{tc.entry}, Construct: constructRustBinTarget}
			err := validateWitness(indexInspection(insp), w)
			if (err == nil) != tc.valid {
				t.Errorf("valid=%t, validation error=%v", tc.valid, err)
			}
		})
	}
}

func TestClippedCargoCannotInventAutodiscoveryDefaults(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		cargo := "dependencies.clap='4'\n[package]\nname='tool'\nedition='2021'\n"
		if explicit {
			cargo += "autobins=true\n"
		}
		insp := InspectionFromSources("Rust", []string{"Cargo.toml", "src/main.rs"}, map[string]string{"Cargo.toml": cargo})
		insp.Files[0].Truncated, insp.Files[0].Size = true, 1000
		got := Classify(insp)
		if explicit && (got.Kind != apimodel.KindCLI || got.Status != VerdictResolved) {
			t.Errorf("explicit discovery lost a supported positive: %s/%s", got.Kind, got.Status)
		}
		if !explicit && got.Status != VerdictInconclusive {
			t.Errorf("unread autobins setting treated as a default: %s/%s: %v", got.Kind, got.Status, got.Evidence())
		}
	}
}
