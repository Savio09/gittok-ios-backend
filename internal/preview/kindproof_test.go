package preview

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func kindProof(p apimodel.RepoPreview) (apimodel.Claim, bool) {
	for _, c := range claimsIn(p) {
		if strings.HasPrefix(c.Label, "Why this is ") {
			return c, true
		}
	}
	return apimodel.Claim{}, false
}

func allSources(c apimodel.Claim) []apimodel.Source {
	return append([]apimodel.Source{c.Source}, c.AdditionalSources...)
}

// R3: the displayed CLI kind has a visible explanation citing every premise of
// the classifier's basis, and the cited lines match the consumed bytes.
func TestReviewKindExplanationCitesEveryPremise(t *testing.T) {
	in := corpusInput(t, "cli__cli.json")
	p := Build(in)
	proof, ok := kindProof(p)
	if p.Kind != apimodel.KindCLI || !ok {
		t.Fatalf("kind %s without a visible explanation (proof found: %v)", p.Kind, ok)
	}
	if first := p.Sections[0].Claims[0]; first.Label != proof.Label {
		t.Errorf("the kind explanation must lead the preview, got %q first", first.Label)
	}
	if !strings.HasPrefix(proof.Explanation, "Classified as a CLI from the inspected files") {
		t.Errorf("not framed as a classification from inspected evidence: %q", proof.Explanation)
	}
	basis := crawler.Classify(in.Inspection).Basis
	sources := allSources(proof)
	for _, premise := range basis.Premises {
		w := premise.Witness
		covered := false
		for _, s := range sources {
			if s.Kind == apimodel.SourceFile && s.Ref == w.Path && (w.Kind != crawler.WitnessSpan || s.Lines != "") {
				covered = true
			}
		}
		if !covered {
			t.Errorf("premise %s is not cited: %+v", w.String(), sources)
		}
	}
	// Independent audit of the two decisive premises against raw bytes.
	cobra, clause := false, false
	for _, s := range sources {
		if s.Lines == "" {
			continue
		}
		text := consumedLines(t, in, s.Ref, s.Lines)
		cobra = cobra || (s.Ref == "go.mod" && strings.Contains(text, "github.com/spf13/cobra"))
		clause = clause || (strings.HasSuffix(s.Ref, "main.go") && strings.Contains(text, "package main"))
	}
	if !cobra || !clause {
		t.Errorf("decisive premises not cited at their lines (cobra %v, package main %v): %+v", cobra, clause, sources)
	}
	if proof.Limit == "" || !strings.Contains(proof.Limit, "not run") {
		t.Errorf("an inference must state its limit: %q", proof.Limit)
	}
}

const proofSHA = "0123456789abcdef0123456789abcdef01234567"

func rustCLIInput(treeRequest string) Input {
	insp := crawler.InspectionFromSources("Rust", []string{"Cargo.toml", "src/main.rs"}, map[string]string{
		"Cargo.toml": "[package]\nname = \"tool\"\nedition = \"2021\"\n\n[dependencies]\nclap = \"4\"\n",
	})
	insp.Revision = crawler.Revision{Ref: proofSHA, Resolved: true}
	insp.TreeRequest = treeRequest
	return Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r", Metadata: map[string]githubclient.MetadataField{"full_name": {Value: "o/r"}}}, Inspection: insp}
}

// A listing premise is cited by the tree request that produced the listing.
// Without that recorded provenance, or with provenance for another repository
// or commit, the premise cannot be cited and the kind is withheld — while the
// internal classifier result is unchanged.
func TestKindProofNeedsRecordedTreeProvenanceForListingPremises(t *testing.T) {
	good := githubclient.TreeRequestPath("o", "r", proofSHA)
	for name, tc := range map[string]struct {
		request string
		want    apimodel.RepoKind
	}{
		"recorded request":     {good, apimodel.KindCLI},
		"synthetic listing":    {"", apimodel.KindOther},
		"another repository":   {githubclient.TreeRequestPath("o", "other", proofSHA), apimodel.KindOther},
		"another commit":       {githubclient.TreeRequestPath("o", "r", strings.Repeat("a", 40)), apimodel.KindOther},
		"branch, not a commit": {githubclient.TreeRequestPath("o", "r", "main"), apimodel.KindOther},
	} {
		t.Run(name, func(t *testing.T) {
			in := rustCLIInput(tc.request)
			if got := crawler.Classify(in.Inspection); got.Kind != apimodel.KindCLI {
				t.Fatalf("fixture no longer resolves internally: %s", got.Kind)
			}
			p := Build(in)
			if p.Kind != tc.want {
				t.Fatalf("displayed %s, want %s", p.Kind, tc.want)
			}
			proof, ok := kindProof(p)
			if tc.want == apimodel.KindOther {
				if ok {
					t.Errorf("withheld kind still explained: %+v", proof)
				}
				return
			}
			var tree, manifest bool
			for _, s := range allSources(proof) {
				tree = tree || (s.Kind == apimodel.SourceAPI && s.Ref == "src/main.rs" && s.URL == "https://api.github.com"+good)
				manifest = manifest || (s.Kind == apimodel.SourceFile && s.Ref == "Cargo.toml")
			}
			if !tree || !manifest {
				t.Errorf("listing premise or its owning manifest not cited: %+v", allSources(proof))
			}
		})
	}
}

// A bounded absence premise is stated within its scope, with the files and the
// tree it rests on — never as a repository-wide absence.
func TestKindProofStatesAbsenceWithinItsScope(t *testing.T) {
	insp := crawler.InspectionFromSources("Go", []string{"go.mod", "lib.go"}, map[string]string{"go.mod": "module example.com/lib\n", "lib.go": "package lib\n\nfunc F() {}\n"})
	insp.Revision = crawler.Revision{Ref: proofSHA, Resolved: true}
	insp.TreeRequest = githubclient.TreeRequestPath("o", "r", proofSHA)
	p := Build(Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}, Inspection: insp})
	proof, ok := kindProof(p)
	if p.Kind != apimodel.KindLibrary || !ok {
		t.Fatalf("kind %s, proof %v", p.Kind, ok)
	}
	if !strings.Contains(proof.Explanation, "read in full") || strings.Contains(strings.ToLower(proof.Explanation), "the repository has no") {
		t.Errorf("absence not stated within its scope: %q", proof.Explanation)
	}
	var gomod, tree bool
	for _, s := range allSources(proof) {
		gomod = gomod || (s.Kind == apimodel.SourceFile && s.Ref == "go.mod")
		tree = tree || s.Kind == apimodel.SourceAPI
	}
	if !gomod || !tree {
		t.Errorf("absence premise must cite go.mod and the tree: %+v", allSources(proof))
	}
}

// A nested comparison cites the complete listing and every compared component.
func TestKindProofCitesTheComparedComponents(t *testing.T) {
	insp := crawler.InspectionFromSources("TypeScript", []string{"web/package.json", "web/index.html", "lib/package.json"}, map[string]string{
		"web/package.json": `{"dependencies":{"react":"18"}}`,
		"lib/package.json": `{"name":"lib","exports":"./index.js"}`,
	})
	insp.Revision = crawler.Revision{Ref: proofSHA, Resolved: true}
	insp.TreeRequest = githubclient.TreeRequestPath("o", "r", proofSHA)
	p := Build(Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}, Inspection: insp})
	proof, ok := kindProof(p)
	if p.Kind != apimodel.KindFrontend || !ok {
		t.Fatalf("kind %s, proof %v", p.Kind, ok)
	}
	if !strings.Contains(proof.Explanation, `"lib" (library)`) {
		t.Errorf("compared component not named: %q", proof.Explanation)
	}
	refs := map[string]bool{}
	for _, s := range allSources(proof) {
		refs[s.Ref] = true
	}
	if !refs["web/package.json"] || !refs["lib/package.json"] || !refs["web/index.html"] {
		t.Errorf("comparison premises not all cited: %v", refs)
	}
}

func TestClaimKeysDistinguishAdditionalSources(t *testing.T) {
	a := apimodel.Claim{Label: "L", Source: apimodel.Source{Ref: "x"}}
	b := a
	b.AdditionalSources = []apimodel.Source{{Ref: "y"}}
	if keyOf(a) == keyOf(b) {
		t.Error("claims differing in additional sources share a key")
	}
	c := b
	c.AdditionalSources = []apimodel.Source{{Ref: "y"}}
	if keyOf(b) != keyOf(c) {
		t.Error("identical claims have different keys")
	}
}

func githubTreeFor(owner, name string) string {
	return githubclient.TreeRequestPath(owner, name, proofSHA)
}

// SW2 (swarm review): a library's no-command premise cites every consulted
// package-clause file, and an unread one withholds the kind.
func TestKindProofCitesEveryConsultedPackageClause(t *testing.T) {
	files := map[string]string{"go.mod": "module example.com/lib\n", "lib.go": "package lib\nfunc F() {}\n", "cmd/tool/helper.go": "package helper\n"}
	paths := []string{"go.mod", "lib.go", "cmd/tool/helper.go"}
	build := func() apimodel.RepoPreview {
		insp := crawler.InspectionFromSources("Go", paths, files)
		insp.Revision = crawler.Revision{Ref: proofSHA, Resolved: true}
		insp.TreeRequest = githubclient.TreeRequestPath("o", "r", proofSHA)
		return Build(Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}, Inspection: insp})
	}
	p := build()
	proof, ok := kindProof(p)
	if p.Kind != apimodel.KindLibrary || !ok {
		t.Fatalf("kind %s, proof %v", p.Kind, ok)
	}
	cited := map[string]bool{}
	for _, s := range allSources(proof) {
		cited[s.Ref] = true
	}
	if !cited["cmd/tool/helper.go"] || !cited["lib.go"] || !cited["go.mod"] {
		t.Errorf("consulted clause files not all cited: %v", cited)
	}
	if !strings.Contains(proof.Explanation, "package clauses") {
		t.Errorf("the content inference is not described: %q", proof.Explanation)
	}
	delete(files, "cmd/tool/helper.go")
	if p := build(); p.Kind != apimodel.KindOther {
		t.Errorf("an unread consulted file still displays %s", p.Kind)
	}
}

// SW4 (swarm review): a truncated listing never yields a proof that the tree
// shows no app shell.
func TestKindProofAbstainsWhenTheListingCannotShowAbsence(t *testing.T) {
	for name, scope := range map[string]crawler.TreeScope{"truncated": {Truncated: true}, "depth-limited": {DepthLimited: true, MaxDepth: 8}} {
		insp := crawler.InspectionFromSources("JavaScript", []string{"package.json"}, map[string]string{"package.json": `{"main":"./index.js"}`})
		insp.Revision = crawler.Revision{Ref: proofSHA, Resolved: true}
		insp.TreeRequest = githubclient.TreeRequestPath("o", "r", proofSHA)
		insp.Scope = scope
		p := Build(Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}, Inspection: insp})
		if _, ok := kindProof(p); p.Kind != apimodel.KindOther || ok {
			t.Errorf("%s listing: displayed %s with proof %v", name, p.Kind, ok)
		}
	}
}

// SW3 (swarm review): a comparison cites why each role-bearing component was
// set aside; the omitted root dependency changes the result when removed.
func TestKindProofCitesEachSupportingExclusion(t *testing.T) {
	files := map[string]string{
		"package.json":              `{"private":true,"workspaces":["apps/*"],"dependencies":{"sidecar":"*"}}`,
		"apps/web/package.json":     `{"name":"web","dependencies":{"react":"19"}}`,
		"apps/sidecar/package.json": `{"name":"sidecar","dependencies":{"express":"5"}}`,
	}
	paths := []string{"package.json", "apps/web/package.json", "apps/web/index.html", "apps/sidecar/package.json"}
	build := func() apimodel.RepoPreview {
		insp := crawler.InspectionFromSources("JavaScript", paths, files)
		insp.Revision = crawler.Revision{Ref: proofSHA, Resolved: true}
		insp.TreeRequest = githubclient.TreeRequestPath("o", "r", proofSHA)
		return Build(Input{Repo: githubclient.Repo{Owner: "o", Name: "r", FullName: "o/r"}, Inspection: insp})
	}
	p := build()
	proof, ok := kindProof(p)
	if p.Kind != apimodel.KindFrontend || !ok {
		t.Fatalf("kind %s, proof %v", p.Kind, ok)
	}
	cited := map[string]bool{}
	for _, s := range allSources(proof) {
		cited[s.Ref] = true
	}
	if !cited["package.json"] || !cited["apps/sidecar/package.json"] {
		t.Errorf("exclusion premises not cited: %v", cited)
	}
	if !strings.Contains(proof.Explanation, `"apps/sidecar" (backend)`) {
		t.Errorf("exclusion not stated: %q", proof.Explanation)
	}
	files["package.json"] = `{"private":true,"workspaces":["apps/*"]}`
	if p := build(); p.Kind != apimodel.KindOther {
		t.Errorf("without the cited dependency the result should change, got %s", p.Kind)
	}
}
