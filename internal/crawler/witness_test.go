package crawler

import (
	"strings"
	"testing"
)

// Task 10a (R8): witnesses are validated against the inspected content. A real
// path, or a quote that matches some bytes, establishes provenance only; the
// validator re-derives the claimed field or construct, so an inspected but
// unrelated file, an omitted region or a mutated witness is rejected.

const witnessMain = "package main\n\nimport \"net/http\"\n\n// http.ListenAndServe is mentioned here only in a comment.\nfunc main() {\n\thttp.ListenAndServe(\":80\", nil)\n}\n"

func witnessIndex() *inspIndex {
	return indexInspection(Inspection{
		Paths: []string{"package.json", "go.mod", "cmd/api/main.go", "cmd/api/util.go", "cmd/api/imports.go", "pyproject.toml", "requirements.txt",
			"Dockerfile", "big.go", "index.html", "android/app/build.gradle", "ios/Runner/Info.plist", "unread.go"},
		Files: []SourceFile{
			{Path: "package.json", Content: `{"dependencies":{"express":"^4.19.0"},"bin":"cli.js","private":true}`},
			{Path: "go.mod", Content: "module example.com/api\n\nrequire github.com/spf13/cobra v1.8.0\n"},
			{Path: "cmd/api/main.go", Content: witnessMain},
			{Path: "cmd/api/util.go", Content: "package main\n\nfunc helper() {}\n"},
			{Path: "cmd/api/imports.go", Content: "package main\n\nimport \"example.com/api/internal/srv\"\n\nvar _ = srv.Run\n"},
			{Path: "pyproject.toml", Content: "[project]\nname='svc'\ndependencies=['flask>=3.0']\n[project.optional-dependencies]\ndocs=['sphinx']\n[project.scripts]\nsvc='svc:main'\n"},
			{Path: "requirements.txt", Content: "Flask==3.0.0\n"},
			{Path: "Dockerfile", Content: "FROM alpine\nEXPOSE 8080\n"},
			{Path: "big.go", Content: "package big\n", Size: 4096, Truncated: true},
		},
	})
}

func callSpan(t *testing.T) (int, int) {
	t.Helper()
	g := analyzeGoFile(SourceFile{Path: "cmd/api/main.go", Content: witnessMain})
	if len(g.serverCalls) != 1 {
		t.Fatalf("fixture should hold one server call, got %+v", g.serverCalls)
	}
	return g.serverCalls[0].start, g.serverCalls[0].end
}

func TestValidWitnessesAreAccepted(t *testing.T) {
	st := witnessIndex()
	start, end := callSpan(t)
	pkg := analyzeGoFile(SourceFile{Path: "cmd/api/main.go", Content: witnessMain})
	expose := strings.Index("FROM alpine\nEXPOSE 8080\n", "EXPOSE")
	valid := map[string]Witness{
		"npm runtime dependency":   {Kind: WitnessField, Path: "package.json", Field: []string{"dependencies", "express"}, Value: "^4.19.0"},
		"npm bin":                  {Kind: WitnessField, Path: "package.json", Field: []string{"bin"}, Value: "cli.js"},
		"npm private flag":         {Kind: WitnessField, Path: "package.json", Field: []string{"private"}, Value: "true"},
		"go module path":           {Kind: WitnessField, Path: "go.mod", Field: []string{"module"}, Value: "example.com/api"},
		"go requirement with dots": {Kind: WitnessField, Path: "go.mod", Field: []string{"require", "github.com/spf13/cobra"}, Value: "v1.8.0"},
		"python mandatory dep":     {Kind: WitnessField, Path: "pyproject.toml", Field: []string{"project", "dependencies", "flask"}, Value: "flask>=3.0"},
		"python optional group":    {Kind: WitnessField, Path: "pyproject.toml", Field: []string{"project", "optional-dependencies", "docs", "sphinx"}},
		"python script":            {Kind: WitnessField, Path: "pyproject.toml", Field: []string{"project", "scripts", "svc"}, Value: "svc:main"},
		"requirements line":        {Kind: WitnessField, Path: "requirements.txt", Field: []string{"requirements", "flask"}, Value: "Flask==3.0.0"},
		"server call span":         {Kind: WitnessSpan, Path: "cmd/api/main.go", Start: start, End: end, Quote: witnessMain[start:end], Construct: constructGoServerCall},
		"package clause span":      {Kind: WitnessSpan, Path: "cmd/api/main.go", Start: pkg.pkgStart, End: pkg.pkgEnd, Quote: "main", Construct: constructGoPackageMain},
		"dockerfile EXPOSE":        {Kind: WitnessSpan, Path: "Dockerfile", Start: expose, End: expose + len("EXPOSE 8080"), Construct: constructDockerExpose},
		"index.html listing":       {Kind: WitnessListing, Paths: []string{"index.html"}, Construct: constructIndexHTML},
		"native shells listing":    {Kind: WitnessListing, Paths: []string{"android/app/build.gradle", "ios/Runner/Info.plist"}, Construct: constructNativeShells},
		"no-server scope":          {Kind: WitnessScope, Paths: []string{"go.mod", "cmd/api/util.go"}, Construct: constructGoNoServer},
		"complete-read scope":      {Kind: WitnessScope, Paths: []string{"pyproject.toml"}, Construct: constructCompleteReads},
	}
	for name, w := range valid {
		if err := validateWitness(st, w); err != nil {
			t.Errorf("%s: rejected a valid witness: %v", name, err)
		}
	}
}

func TestInvalidWitnessesAreRejected(t *testing.T) {
	st := witnessIndex()
	start, end := callSpan(t)
	comment := strings.Index(witnessMain, "http.ListenAndServe is mentioned")
	cases := map[string]struct {
		w    Witness
		want string // a fragment of the rejection reason
	}{
		"missing witness": {Witness{}, "no witness"},
		"path that was never read": {Witness{Kind: WitnessField, Path: "unread.go", Field: []string{"x"}},
			"was not read"},
		"path absent from the inspection": {Witness{Kind: WitnessField, Path: "nowhere/package.json", Field: []string{"dependencies", "express"}},
			"was not read"},
		"field value that differs": {Witness{Kind: WitnessField, Path: "package.json", Field: []string{"dependencies", "express"}, Value: "^5.0.0"},
			`is "^4.19.0"`},
		"field that does not exist": {Witness{Kind: WitnessField, Path: "package.json", Field: []string{"devDependencies", "express"}},
			"has no field"},
		"optional dep claimed as mandatory": {Witness{Kind: WitnessField, Path: "pyproject.toml", Field: []string{"project", "dependencies", "sphinx"}},
			"has no field"},
		"mismatched quote": {Witness{Kind: WitnessSpan, Path: "cmd/api/main.go", Start: start, End: end, Quote: "http.Serve(l, nil)", Construct: constructGoServerCall},
			"quote does not match"},
		"span out of range": {Witness{Kind: WitnessSpan, Path: "cmd/api/main.go", Start: start, End: len(witnessMain) + 10, Construct: constructGoServerCall},
			"outside the"},
		"span in the unread remainder of a clipped file": {Witness{Kind: WitnessSpan, Path: "big.go", Start: 20, End: 40, Construct: constructGoServerCall},
			"outside the"},
		"matching words in a comment are not the construct": {Witness{Kind: WitnessSpan, Path: "cmd/api/main.go", Start: comment, End: comment + len("http.ListenAndServe"), Quote: "http.ListenAndServe", Construct: constructGoServerCall},
			"no call resolved"},
		"inspected but unrelated file": {Witness{Kind: WitnessSpan, Path: "cmd/api/util.go", Start: 15, End: 29, Construct: constructGoServerCall},
			"no call resolved"},
		"AST witness without its span": {Witness{Kind: WitnessSpan, Path: "cmd/api/main.go", Construct: constructGoServerCall},
			"omits its required source span"},
		"package clause claimed elsewhere": {Witness{Kind: WitnessSpan, Path: "cmd/api/main.go", Start: start, End: end, Construct: constructGoPackageMain},
			"not its package clause"},
		"listing path that is not listed": {Witness{Kind: WitnessListing, Paths: []string{"public/index.html"}, Construct: constructIndexHTML},
			"not in the listing"},
		"listing that does not establish the construct": {Witness{Kind: WitnessListing, Paths: []string{"android/app/build.gradle"}, Construct: constructNativeShells},
			"do not establish"},
		"scope over a clipped file": {Witness{Kind: WitnessScope, Paths: []string{"big.go"}, Construct: constructCompleteReads},
			"read in full"},
		"no-server scope over a file with a server call": {Witness{Kind: WitnessScope, Paths: []string{"go.mod", "cmd/api/main.go"}, Construct: constructGoNoServer},
			"contains a server call"},
		"no-server scope without its go.mod": {Witness{Kind: WitnessScope, Paths: []string{"cmd/api/util.go"}, Construct: constructGoNoServer},
			"needs the module's go.mod"},
		"no-server scope over an own-module import": {Witness{Kind: WitnessScope, Paths: []string{"go.mod", "cmd/api/imports.go"}, Construct: constructGoNoServer},
			"from its own module"},
	}
	for name, c := range cases {
		err := validateWitness(st, c.w)
		if err == nil {
			t.Errorf("%s: accepted an unsupported witness", name)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: rejection %q does not explain %q", name, err, c.want)
		}
	}
}

// A covering-listing premise is re-checked against the listing's real scope.
func TestCoveringListingScopeIsRechecked(t *testing.T) {
	w := Witness{Kind: WitnessScope, Paths: []string{"Cargo.toml"}, Construct: constructCoveringListing, Depth: 3}
	files := []SourceFile{{Path: "Cargo.toml", Content: "[package]\nname='x'\n"}}
	if err := validateWitness(indexInspection(Inspection{Paths: []string{"Cargo.toml"}, Files: files}), w); err != nil {
		t.Errorf("complete listing: %v", err)
	}
	if err := validateWitness(indexInspection(Inspection{Paths: []string{"Cargo.toml"}, Files: files, Scope: TreeScope{Truncated: true}}), w); err == nil {
		t.Errorf("a truncated listing cannot back a covering premise")
	}
}

// Task 10b: every observation the classifier emits — primary or secondary —
// carries a witness, and that witness validates against the inspection it came
// from. Run over a spread of fixtures covering each detector.
func TestEveryEmittedObservationHasAValidWitness(t *testing.T) {
	fixtures := map[string]Inspection{
		"go backend + ui": InspectionFromSources("Go", []string{"go.mod", "main.go", "ui/package.json", "ui/index.html", "Dockerfile"}, map[string]string{
			"go.mod": "module example.com/app\n", "main.go": "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":80\",nil)}\n",
			"ui/package.json": `{"private":true,"dependencies":{"react":"18"}}`, "Dockerfile": "FROM alpine\nEXPOSE 80\n"}),
		"go cli (cobra)": InspectionFromSources("Go", []string{"go.mod", "main.go"}, map[string]string{
			"go.mod": "module example.com/t\n\nrequire github.com/spf13/cobra v1.8.0\n", "main.go": "package main\nfunc main(){}\n"}),
		"go cli (no server)": InspectionFromSources("Go", []string{"go.mod", "cmd/t/main.go"}, map[string]string{
			"go.mod": "module example.com/t\n", "cmd/t/main.go": "package main\nfunc main(){}\n"}),
		"go library": InspectionFromSources("Go", []string{"go.mod", "lib.go"}, map[string]string{"go.mod": "module example.com/l\n", "lib.go": "package l\n"}),
		"go server via gin": InspectionFromSources("Go", []string{"go.mod", "main.go"}, map[string]string{
			"go.mod": "module example.com/s\n\nrequire github.com/gin-gonic/gin v1.9.1\n", "main.go": "package main\nfunc main(){}\n"}),
		"expo + flutter + shells": InspectionFromSources("", []string{"package.json", "app/pubspec.yaml", "android/x", "ios/y"}, map[string]string{
			"package.json": `{"dependencies":{"expo":"51","react-native":"0.74"}}`, "app/pubspec.yaml": "name: a\ndependencies:\n  flutter:\n    sdk: flutter\n"}),
		"npm library + cli": InspectionFromSources("TypeScript", []string{"package.json", "tools/cli/package.json"}, map[string]string{
			"package.json": `{"main":"dist/index.js","peerDependencies":{"react":"18"}}`, "tools/cli/package.json": `{"bin":"cli.js"}`}),
		"node backend": InspectionFromSources("", []string{"package.json"}, map[string]string{"package.json": `{"dependencies":{"express":"^4"}}`}),
		"python": InspectionFromSources("Python", []string{"pyproject.toml", "svc/requirements.txt", "cli/pyproject.toml"}, map[string]string{
			"pyproject.toml": "[project]\nname='lib'\n", "svc/requirements.txt": "Flask==3.0.0\n", "cli/pyproject.toml": "[project]\nname='c'\n[project.scripts]\nc='c:main'\n"}),
		"rust": InspectionFromSources("Rust", []string{"Cargo.toml", "src/lib.rs", "bin/Cargo.toml", "bin/src/main.rs", "api/Cargo.toml", "api/src/main.rs"}, map[string]string{
			"Cargo.toml": "[package]\nname='core'\n", "bin/Cargo.toml": "[package]\nname='b'\n[dependencies]\nclap='4'\n",
			"api/Cargo.toml": "[package]\nname='api'\n[dependencies]\naxum='0.7'\n"}),
	}
	for name, insp := range fixtures {
		st := indexInspection(insp)
		got := Classify(insp)
		all := append(append([]Observation(nil), got.Observations...), got.Secondary...)
		if len(all) == 0 {
			t.Errorf("%s: no observations emitted", name)
		}
		for _, o := range all {
			if err := validateWitness(st, o.Witness); err != nil {
				t.Errorf("%s: emitted observation %q does not validate: %v", name, o.Detail, err)
			}
		}
	}
}

// Invalid support never reaches the verdict: a mutated decisive witness
// withdraws its evidence group and the verdict is re-resolved, an injected
// fabricated candidate is rejected without affecting anything else, and an
// invalid context observation is dropped while its supported role stands.
func TestRejectedEvidenceIsReResolved(t *testing.T) {
	server := InspectionFromSources("Go", []string{"go.mod", "main.go", "Dockerfile"}, map[string]string{
		"go.mod":     "module example.com/api\n",
		"main.go":    "package main\nimport \"net/http\"\nfunc main(){ http.ListenAndServe(\":80\", nil) }\n",
		"Dockerfile": "FROM alpine\nEXPOSE 80\n",
	})
	mutate := func(kind string, f func(*Observation)) candidateHook {
		return func(root string, cands []candidate) []candidate {
			for ci := range cands {
				for gi := range cands[ci].groups {
					for oi := range cands[ci].groups[gi] {
						o := &cands[ci].groups[gi][oi]
						if o.Witness.Construct == kind || strings.Join(o.Witness.Field, ".") == kind {
							f(o)
						}
					}
				}
			}
			return cands
		}
	}

	if base := Classify(server); base.Kind != "backend" || base.Status != VerdictResolved {
		t.Fatalf("baseline: got %q/%q", base.Kind, base.Status)
	}

	cases := map[string]candidateHook{
		"mismatched quote":     mutate(constructGoServerCall, func(o *Observation) { o.Witness.Quote = "http.Serve(l, nil)" }),
		"out-of-range span":    mutate(constructGoServerCall, func(o *Observation) { o.Witness.End = 10_000 }),
		"invalid path":         mutate(constructGoServerCall, func(o *Observation) { o.Witness.Path = "cmd/other/main.go" }),
		"missing span":         mutate(constructGoServerCall, func(o *Observation) { o.Witness.Start, o.Witness.End = 0, 0 }),
		"missing witness":      mutate(constructGoServerCall, func(o *Observation) { o.Witness = Witness{} }),
		"unrelated construct":  mutate(constructGoServerCall, func(o *Observation) { o.Witness.Start, o.Witness.End, o.Witness.Quote = 0, 7, "package" }),
		"wrong package clause": mutate(constructGoPackageMain, func(o *Observation) { o.Witness.Start++ }),
	}
	for name, hook := range cases {
		t.Run("decisive "+name, func(t *testing.T) {
			got := classify(server, hook)
			if got.Kind == "backend" && got.Status == VerdictResolved {
				t.Fatalf("backend survived invalid decisive evidence: %v", got.Evidence())
			}
			if !limitationsContain(got.Limitations, "rejected backend evidence") {
				t.Errorf("the rejection was not recorded: %v", got.Limitations)
			}
			for _, o := range append(got.Observations, got.Secondary...) {
				if o.Witness.Construct == constructGoServerCall {
					t.Errorf("rejected evidence leaked into the result: %+v", o)
				}
			}
		})
	}

	t.Run("fabricated candidate is rejected and changes nothing", func(t *testing.T) {
		fake := func(root string, cands []candidate) []candidate {
			o := fieldObs("package.json", []string{"dependencies", "react"}, "18", "declares \"react\" in dependencies")
			o.Component, o.Role = root, "frontend"
			return append([]candidate{{kind: "frontend", groups: []evidenceGroup{{o}}}}, cands...)
		}
		got := classify(server, fake)
		if got.Kind != "backend" || got.Status != VerdictResolved {
			t.Errorf("a fabricated frontend candidate changed the verdict: %q/%q", got.Kind, got.Status)
		}
		if !limitationsContain(got.Limitations, "rejected frontend evidence") {
			t.Errorf("the fabricated candidate's rejection was not recorded: %v", got.Limitations)
		}
	})

	t.Run("invalid context is dropped; the role stands", func(t *testing.T) {
		got := classify(server, mutate(constructDockerExpose, func(o *Observation) { o.Witness.Start, o.Witness.End = 0, 4 }))
		if got.Kind != "backend" || got.Status != VerdictResolved {
			t.Fatalf("an invalid context observation changed the verdict: %q/%q", got.Kind, got.Status)
		}
		for _, o := range got.Observations {
			if o.Path == "Dockerfile" {
				t.Errorf("rejected context leaked into the result: %+v", o)
			}
		}
		if !limitationsContain(got.Limitations, "rejected supporting backend evidence") {
			t.Errorf("the context rejection was not recorded: %v", got.Limitations)
		}
	})

	t.Run("one rejected support of several leaves the others", func(t *testing.T) {
		mobile := InspectionFromSources("", []string{"package.json"}, map[string]string{
			"package.json": `{"dependencies":{"expo":"51","react-native":"0.74"}}`,
		})
		got := classify(mobile, mutate("dependencies.expo", func(o *Observation) { o.Witness.Value = "49" }))
		if got.Kind != "mobile" || got.Status != VerdictResolved {
			t.Fatalf("react-native alone should still support mobile: %q/%q", got.Kind, got.Status)
		}
		for _, o := range got.Observations {
			if strings.Contains(o.Detail, `"expo"`) {
				t.Errorf("the rejected expo evidence leaked: %+v", o)
			}
		}
	})
}
