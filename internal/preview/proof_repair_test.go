package preview

import (
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
)

func TestGoNoCommandDisplayNeedsCompleteListing(t *testing.T) {
	for name, scope := range map[string]crawler.TreeScope{
		"complete": {}, "depth three": {DepthLimited: true, MaxDepth: 3}, "truncated": {Truncated: true},
	} {
		t.Run(name, func(t *testing.T) {
			in := synthetic([]string{"go.mod", "lib.go"}, map[string]string{
				"go.mod": "module example.com/lib\n", "lib.go": "package lib\nfunc F() {}\n",
			})
			in.Inspection.Scope = scope
			p := Build(in)
			_, proof := kindProof(p)
			if scope.Complete() {
				if p.Kind != apimodel.KindLibrary || !proof {
					t.Fatalf("complete library lost its proof: %s/%v", p.Kind, proof)
				}
			} else if p.Kind != apimodel.KindOther || proof {
				t.Errorf("incomplete listing displayed an unbounded no-command inference: %s/%v", p.Kind, proof)
			}
		})
	}
	full := synthetic([]string{"go.mod", "lib.go", "cmd/tool/sub/entry.go"}, map[string]string{
		"go.mod": "module example.com/lib\n", "lib.go": "package lib\nfunc F() {}\n",
		"cmd/tool/sub/entry.go": "package main\nfunc main() {}\n",
	})
	if p := Build(full); p.Kind != apimodel.KindCLI {
		t.Fatalf("the unseen command counterexample did not resolve CLI: %s", p.Kind)
	}
}

func TestGoNoCommandWordingDoesNotExcludeOtherModules(t *testing.T) {
	in := synthetic([]string{"go.mod", "lib.go", "cmd/separate/go.mod", "cmd/separate/helper.go"}, map[string]string{
		"go.mod": "module example.com/lib\n", "lib.go": "package lib\nfunc F() {}\n",
		"cmd/separate/go.mod": "module example.com/separate\n", "cmd/separate/helper.go": "package helper\n",
	})
	proof, ok := kindProof(Build(in))
	if !ok {
		t.Fatal("complete root-library fixture must retain its scoped proof")
	}
	if strings.Contains(proof.Explanation, "lists no other Go file under cmd/") {
		t.Error("the proof denied an actually listed file owned by a separate module")
	}
}

func excludedRoleInput() Input {
	return synthetic([]string{"package.json", "apps/web/package.json", "apps/web/index.html", "apps/sidecar/package.json"}, map[string]string{
		"package.json":              "{\n \"private\":true,\n \"workspaces\":[\"apps/*\"],\n \"dependencies\":{\"sidecar\":\"*\"}\n}",
		"apps/web/package.json":     "{\n \"name\":\"web\",\n \"dependencies\":{\"react\":\"19\"}\n}",
		"apps/sidecar/package.json": "{\n \"name\":\"sidecar\",\n \"dependencies\":{\"express\":\"5\"}\n}",
	})
}

func TestExcludedComponentRoleCitesItsOwnPremises(t *testing.T) {
	in := excludedRoleInput()
	proof, ok := kindProof(Build(in))
	if !ok || !strings.Contains(proof.Explanation, `"apps/sidecar" (backend)`) {
		t.Fatal("fixture must retain the explained excluded sidecar")
	}
	var name, role bool
	for _, s := range allSources(proof) {
		if s.Ref != "apps/sidecar/package.json" || s.Kind != apimodel.SourceFile || s.Lines == "" {
			continue
		}
		text := consumedLines(t, in, s.Ref, s.Lines)
		name = name || strings.Contains(text, `"name":"sidecar"`)
		role = role || strings.Contains(text, `"express":"5"`)
	}
	if !name || !role {
		t.Errorf("excluded role lacks precise name and framework citations: name=%v role=%v sources=%+v", name, role, allSources(proof))
	}
}

func TestExcludedRoleWithoutItsBasisWithholdsTheProof(t *testing.T) {
	in := excludedRoleInput()
	cls := crawler.Classify(in.Inspection)
	if cls.Basis == nil || len(cls.Basis.Excluded) != 1 || len(cls.Basis.Excluded[0].RolePremises) == 0 {
		t.Fatal("fixture needs a role-bearing excluded component with its own basis")
	}
	cls.Basis.Excluded[0].RolePremises = nil
	if _, ok := kindClaim(in, cls); ok {
		t.Error("displayed a role label without its basis")
	}
}
