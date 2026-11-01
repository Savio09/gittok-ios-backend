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
		Paths: []string{"package.json", "go.mod", "cmd/api/main.go", "cmd/api/util.go", "pyproject.toml", "requirements.txt",
			"Dockerfile", "big.go", "index.html", "android/app/build.gradle", "ios/Runner/Info.plist", "unread.go"},
		Files: []SourceFile{
			{Path: "package.json", Content: `{"dependencies":{"express":"^4.19.0"},"bin":"cli.js","private":true}`},
			{Path: "go.mod", Content: "module example.com/api\n\nrequire github.com/spf13/cobra v1.8.0\n"},
			{Path: "cmd/api/main.go", Content: witnessMain},
			{Path: "cmd/api/util.go", Content: "package main\n\nfunc helper() {}\n"},
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
		"no-server scope over a file with a server call": {Witness{Kind: WitnessScope, Paths: []string{"cmd/api/main.go"}, Construct: constructGoNoServer},
			"contains a server call"},
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
