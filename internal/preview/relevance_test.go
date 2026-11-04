package preview

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Selection policy under test (G17 review, R1/R2): product evidence leads,
// dependency claims are bounded (at most 8, at most 2 lower-priority claims),
// supporting tools never stand in for the product, and nothing depends on the
// repository's name or the input order.

var auditLowCategories = map[string]bool{"devDependencies": true, "peerDependencies": true, "optionalDependencies": true, "require (indirect)": true, "dev-dependencies": true, "build-dependencies": true, "dev_dependencies": true}

type selection struct {
	deps, low, helpers, product int
	labels                      map[string]bool
}

// measure classifies the rendered claims independently of the ranker, from
// their wording, references and the crawler's path hint.
func measure(p apimodel.RepoPreview) selection {
	s := selection{labels: map[string]bool{}}
	for _, c := range claimsIn(p) {
		s.labels[c.Label] = true
		helper := c.Source.Kind == apimodel.SourceFile && crawler.SupportingPath(c.Source.Ref)
		if helper {
			s.helpers++
		}
		if !strings.HasPrefix(c.Label, "Declares ") || strings.HasPrefix(c.Label, "Declares command ") {
			continue
		}
		s.deps++
		low := false
		for cat := range auditLowCategories {
			low = low || strings.Contains(c.Explanation, "in \""+cat+"\"")
		}
		if low || helper {
			s.low++
		} else {
			s.product++
		}
	}
	return s
}

func TestReviewCLIProductEvidenceLeads(t *testing.T) {
	p := Build(corpusInput(t, "cli__cli.json"))
	s := measure(p)
	if !s.labels["Declares github.com/spf13/cobra"] {
		t.Error("the decisive CLI framework declaration was displaced")
	}
	if s.deps > 8 || s.low > 2 {
		t.Errorf("dependency budget exceeded: %d declarations, %d lower-priority", s.deps, s.low)
	}
	if s.helpers > 2 || s.helpers > s.product {
		t.Errorf("supporting tools dominate: %d helper claims vs %d product declarations", s.helpers, s.product)
	}
	for _, c := range claimsIn(p) {
		if c.Label == "Contains net/http call" && crawler.SupportingPath(c.Source.Ref) {
			t.Errorf("a helper's HTTP call stands in for the product walkthrough: %s", c.Source.Ref)
		}
	}
}

func TestReviewPocketBaseKeepsMainAndUIEvidence(t *testing.T) {
	p := Build(corpusInput(t, "pocketbase__pocketbase.json"))
	s := measure(p)
	rootDirect := 0
	for _, c := range claimsIn(p) {
		if c.Source.Ref == "go.mod" && strings.HasPrefix(c.Label, "Declares ") && strings.Contains(c.Explanation, `in "require"`) {
			rootDirect++
		}
	}
	if rootDirect < 3 {
		t.Errorf("main module's direct declarations displaced: %d", rootDirect)
	}
	if !s.labels["Declares command build"] || !s.labels["Declares command dev"] {
		t.Errorf("UI build/dev commands displaced: %v", s.labels)
	}
	if s.deps > 8 || s.low > 2 {
		t.Errorf("dependency budget exceeded: %d declarations, %d lower-priority", s.deps, s.low)
	}
}

func metadataInput() Input {
	in := inputForTest()
	in.Repo.Stars, in.Repo.License, in.Repo.Language = 61611, "MIT", "Go"
	in.Repo.PushedAt = time.Date(2026, 10, 26, 18, 7, 11, 0, time.UTC)
	in.Repo.Metadata["stargazers_count"] = githubclient.MetadataField{Value: "61611"}
	in.Repo.Metadata["license.spdx_id"] = githubclient.MetadataField{Value: "MIT"}
	in.Repo.Metadata["pushed_at"] = githubclient.MetadataField{Value: "2026-10-26T18:07:11Z"}
	in.Repo.Metadata["language"] = githubclient.MetadataField{Value: "Go"}
	return in
}

func TestCardMetadataBecomesSourcedMetrics(t *testing.T) {
	p := Build(metadataInput())
	for _, c := range claimsIn(p) {
		switch c.Source.Ref {
		case "stargazers_count", "license.spdx_id", "pushed_at", "full_name":
			t.Errorf("card metadata spent a claim slot: %s", c.Label)
		}
	}
	got := map[string]apimodel.Metric{}
	for _, m := range p.Metrics {
		got[m.Key] = m
	}
	for key, want := range map[string]struct{ value, ref string }{
		"stars":     {"61611", "stargazers_count"},
		"license":   {"MIT", "license.spdx_id"},
		"last_push": {"2026-10-26T18:07:11Z", "pushed_at"},
	} {
		m, ok := got[key]
		if !ok || m.Value != want.value || m.Source == nil || m.Source.Kind != apimodel.SourceMetadata || m.Source.Ref != want.ref || m.Source.URL == "" {
			t.Errorf("metric %s = %+v", key, m)
		}
	}
	if m := got["last_push"]; strings.Contains(strings.ToLower(m.Label), "commit") {
		t.Errorf("pushed_at labelled as a commit time: %q", m.Label)
	}
	// Missing provider values never become zeros.
	bare := inputForTest()
	if p := Build(bare); len(p.Metrics) != 0 {
		t.Errorf("missing metadata invented metrics: %+v", p.Metrics)
	}
}

func withTree(in Input) Input {
	in.Inspection.Revision = crawler.Revision{Ref: testSHA, Resolved: true}
	in.Inspection.TreeRequest = githubclient.TreeRequestPath(in.Repo.Owner, in.Repo.Name, testSHA)
	return in
}

func synthetic(paths []string, files map[string]string) Input {
	in := inputForTest()
	in.Inspection = crawler.InspectionFromSources("", paths, files)
	return withTree(in)
}

func depLabels(p apimodel.RepoPreview) []string {
	var out []string
	for _, c := range claimsIn(p) {
		if strings.HasPrefix(c.Label, "Declares ") && !strings.HasPrefix(c.Label, "Declares command ") {
			out = append(out, c.Label+" @"+c.Source.Ref)
		}
	}
	return out
}

func TestRootAggregatorDefersToItsMemberProduct(t *testing.T) {
	p := Build(synthetic([]string{"package.json", "packages/web/package.json", "packages/web/index.html"}, map[string]string{
		"package.json":              `{"private":true,"workspaces":["packages/*"],"devDependencies":{"eslint":"9","prettier":"3","typescript":"5","vitest":"2"}}`,
		"packages/web/package.json": `{"name":"web","dependencies":{"react":"18","react-dom":"18"}}`,
	}))
	deps := depLabels(p)
	if len(deps) == 0 || !strings.HasSuffix(deps[0], "@packages/web/package.json") {
		t.Errorf("the member product's declarations must lead: %v", deps)
	}
	if s := measure(p); s.low > 2 {
		t.Errorf("aggregator tooling exceeded the lower-priority budget: %v", deps)
	}
}

func TestLibraryRuntimeDependenciesOutrankDevelopmentTools(t *testing.T) {
	p := Build(synthetic([]string{"package.json", "index.js"}, map[string]string{
		"package.json": `{"name":"lib","exports":"./index.js","dependencies":{"lodash":"4"},"devDependencies":{"eslint":"9","prettier":"3","typescript":"5","vitest":"2"}}`,
	}))
	s := measure(p)
	if !s.labels["Declares lodash"] || s.low > 2 {
		t.Errorf("runtime dependency displaced or dev tools unbounded: %v", depLabels(p))
	}
}

// A repository whose only code lives under a tooling-like path is still
// described by that code: path names are hints, not proof.
func TestToolOnlyRepositoryIsNotDemotedByItsPath(t *testing.T) {
	p := Build(synthetic([]string{"tools/app/go.mod", "tools/app/main.go"}, map[string]string{
		"tools/app/go.mod":  "module example.com/app\n\nrequire (\n\tgithub.com/spf13/cobra v1.8.0\n\tgithub.com/spf13/pflag v1.0.5\n\tgolang.org/x/term v0.20.0\n)\n",
		"tools/app/main.go": "package main\n\nimport \"github.com/spf13/cobra\"\n\nfunc main() { _ = cobra.Command{} }\n",
	}))
	if deps := depLabels(p); len(deps) != 3 {
		t.Errorf("the repository's only code was treated as a helper: %v", deps)
	}
}

func TestUnknownEcosystemStillYieldsAttributedFacts(t *testing.T) {
	p := Build(synthetic([]string{"README.md", "Makefile"}, map[string]string{"README.md": "# Tool\n\nDoes a thing.\n", "Makefile": "all:\n\techo hi\n"}))
	if p.Kind != apimodel.KindOther || !measure(p).labels["README passage"] {
		t.Errorf("unknown ecosystem: %s %v", p.Kind, measure(p).labels)
	}
}

func TestImportedDependenciesRespectModuleBoundaries(t *testing.T) {
	gomod := "module example.com/app\n\nrequire (\n"
	for i := 1; i <= 15; i++ {
		gomod += fmt.Sprintf("\texample.com/d%02d v1.0.0\n", i)
	}
	gomod += "\tgithub.com/zz/b v1.0.0\n\tgithub.com/zz/bc v1.0.0\n)\n"
	p := Build(synthetic([]string{"go.mod", "lib.go"}, map[string]string{
		"go.mod": gomod,
		"lib.go": "package app\n\nimport \"github.com/zz/bc/sub\"\n\nvar _ = sub.X\n",
	}))
	s := measure(p)
	if !s.labels["Declares github.com/zz/bc"] {
		t.Errorf("an imported declaration was displaced: %v", depLabels(p))
	}
	if s.labels["Declares github.com/zz/b"] {
		t.Errorf("github.com/zz/b was ranked as imported by github.com/zz/bc/sub: %v", depLabels(p))
	}
	if s.deps > 8 {
		t.Errorf("dependency cap exceeded: %d", s.deps)
	}
}

// Replaces the earlier 24-slot expectation: two unclassified components with
// 100 declarations each share a bounded dependency budget deterministically.
func TestUnclassifiedComponentsShareABoundedBudget(t *testing.T) {
	deps := map[string]string{}
	for i := 0; i < 100; i++ {
		deps[fmt.Sprintf("dep%02d", i)] = "1"
	}
	body, _ := json.Marshal(map[string]any{"dependencies": deps})
	in := synthetic([]string{"a/package.json", "b/package.json"}, map[string]string{"a/package.json": string(body), "b/package.json": string(body)})
	p := Build(in)
	counts := map[string]int{}
	for _, c := range claimsIn(p) {
		counts[c.Source.Ref]++
	}
	if s := measure(p); s.deps > 8 || counts["a/package.json"] == 0 || counts["b/package.json"] == 0 {
		t.Fatalf("unbounded or starved: %d declarations, %v", s.deps, counts)
	}
	in.Inspection.Files[0], in.Inspection.Files[1] = in.Inspection.Files[1], in.Inspection.Files[0]
	if !reflect.DeepEqual(p, Build(in)) {
		t.Fatal("output depends on input order")
	}
	if last := p.Sections[len(p.Sections)-1]; last.Kind != apimodel.SectionDecision || last.Body == "" {
		t.Fatal("omitted declarations are not disclosed")
	}
}

func TestSelectionIgnoresInputOrderAndRepositoryName(t *testing.T) {
	in := corpusInput(t, "cli__cli.json")
	in.Inspection.TreeRequest = githubclient.TreeRequestPath(in.Repo.Owner, in.Repo.Name, in.Inspection.Revision.Ref)
	base := Build(in)
	shuffled := in
	shuffled.Inspection.Files = append([]crawler.SourceFile(nil), in.Inspection.Files...)
	shuffled.Inspection.Paths = append([]string(nil), in.Inspection.Paths...)
	for i, j := 0, len(shuffled.Inspection.Files)-1; i < j; i, j = i+1, j-1 {
		shuffled.Inspection.Files[i], shuffled.Inspection.Files[j] = shuffled.Inspection.Files[j], shuffled.Inspection.Files[i]
	}
	for i, j := 0, len(shuffled.Inspection.Paths)-1; i < j; i, j = i+1, j-1 {
		shuffled.Inspection.Paths[i], shuffled.Inspection.Paths[j] = shuffled.Inspection.Paths[j], shuffled.Inspection.Paths[i]
	}
	if !reflect.DeepEqual(base, Build(shuffled)) {
		t.Error("selection depends on input order")
	}
	renamed := in
	renamed.Repo.Owner, renamed.Repo.Name, renamed.Repo.FullName = "someone", "else", "someone/else"
	renamed.Repo.Metadata = map[string]githubclient.MetadataField{"full_name": {Value: "someone/else"}, "language": {Value: in.Repo.Language}}
	renamed.Inspection.TreeRequest = githubclient.TreeRequestPath("someone", "else", in.Inspection.Revision.Ref)
	shape := func(p apimodel.RepoPreview) []string {
		var out []string
		for _, c := range claimsIn(p) {
			out = append(out, c.Label+"|"+c.Source.Ref+"|"+c.Source.Lines)
		}
		return out
	}
	if !reflect.DeepEqual(shape(base), shape(Build(renamed))) {
		t.Error("selection depends on the repository's name")
	}
}
