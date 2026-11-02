package crawler

import (
	"math/rand"
	"reflect"
	"testing"
)

// Task 2 (R2): the inventory keeps every manifest with the component that owns
// it, so independent components keep separate sources, entries and dependency
// declarations instead of sharing one slot per manifest basename.

func compByRoot(t *testing.T, comps []*component, root string) *component {
	t.Helper()
	for _, c := range comps {
		if c.root == root {
			return c
		}
	}
	t.Fatalf("no component rooted at %q", root)
	return nil
}

func hasPath(c *component, p string) bool {
	for _, q := range c.paths {
		if q == p {
			return true
		}
	}
	return false
}

func TestInventoryRetainsEveryManifestWithItsOwner(t *testing.T) {
	insp := InspectionFromSources("", []string{
		"package.json", "go.mod", "Cargo.toml", "Dockerfile",
		"ui/package.json", "ui/Dockerfile", "ui/index.html",
		"packages/demo/go.mod", "packages/demo/main.go",
		"crates/core/Cargo.toml", "crates/core/src/lib.rs",
		"tools/Dockerfile",
	}, map[string]string{
		"package.json":           `{"name":"root"}`,
		"go.mod":                 "module example.com/root\n",
		"Cargo.toml":             "[package]\nname='root'\n",
		"ui/package.json":        `{"name":"ui"}`,
		"packages/demo/go.mod":   "module example.com/demo\n",
		"crates/core/Cargo.toml": "[package]\nname='core'\n",
	})
	comps := buildInventory(indexInspection(insp))

	roots := []string{}
	for _, c := range comps {
		roots = append(roots, c.root)
	}
	if want := []string{"", "crates/core", "packages/demo", "ui"}; !reflect.DeepEqual(roots, want) {
		t.Fatalf("component roots = %v, want %v", roots, want)
	}
	owned := map[string][]string{}
	for _, c := range comps {
		for _, m := range c.manifests {
			owned[c.root] = append(owned[c.root], m.path)
		}
	}
	want := map[string][]string{
		"":              {"Cargo.toml", "Dockerfile", "go.mod", "package.json", "tools/Dockerfile"},
		"crates/core":   {"crates/core/Cargo.toml"},
		"packages/demo": {"packages/demo/go.mod"},
		"ui":            {"ui/Dockerfile", "ui/package.json"},
	}
	if !reflect.DeepEqual(owned, want) {
		t.Errorf("manifests by owner =\n %v\nwant\n %v", owned, want)
	}
}

// The two R2 fixtures: a nested app or module keeps its own entry, sources and
// dependency declarations, separate from the root product's.
func TestInventorySeparatesNestedComponents(t *testing.T) {
	ui := buildInventory(indexInspection(InspectionFromSources("TypeScript",
		[]string{"package.json", "src/index.ts", "ui/package.json", "ui/index.html"},
		map[string]string{
			"package.json":    `{"exports":"./dist/index.js","dependencies":{"react":"18"}}`,
			"ui/package.json": `{"private":true,"dependencies":{"react":"18"}}`,
		})))
	root, app := compByRoot(t, ui, ""), compByRoot(t, ui, "ui")
	if hasPath(root, "ui/index.html") || !hasPath(app, "ui/index.html") {
		t.Errorf("ui/index.html must be owned by ui, not the root library")
	}
	if len(root.deps) != 1 || root.deps[0].Source != "package.json" || root.deps[0].Field != "dependencies.react" {
		t.Errorf("root deps = %+v, want only its own react declaration", root.deps)
	}
	if len(app.deps) != 1 || app.deps[0].Source != "ui/package.json" {
		t.Errorf("ui deps = %+v, want only ui/package.json's react declaration", app.deps)
	}

	goInv := buildInventory(indexInspection(InspectionFromSources("Go",
		[]string{"go.mod", "lib.go", "packages/demo/go.mod", "packages/demo/main.go"},
		map[string]string{
			"go.mod":                "module example.com/lib\n",
			"lib.go":                "package lib\n",
			"packages/demo/go.mod":  "module example.com/demo\n\nrequire github.com/gin-gonic/gin v1.9.1\n",
			"packages/demo/main.go": "package main\n",
		})))
	lib, demo := compByRoot(t, goInv, ""), compByRoot(t, goInv, "packages/demo")
	if !hasPath(lib, "lib.go") || hasPath(lib, "packages/demo/main.go") || !hasPath(demo, "packages/demo/main.go") {
		t.Errorf("sources are not attributed to their own module: root=%v demo=%v", lib.paths, demo.paths)
	}
	if len(lib.deps) != 0 || len(demo.deps) != 1 || demo.deps[0].Name != "github.com/gin-gonic/gin" {
		t.Errorf("the demo module's requirement leaked or was lost: root=%+v demo=%+v", lib.deps, demo.deps)
	}
}

// Dependency declarations keep the table they came from, and a manifest keeps
// its read state even when it could not be read.
func TestInventoryKeepsCategoriesAndReadState(t *testing.T) {
	insp := InspectionFromSources("Python", []string{"pyproject.toml", "web/package.json", "web/requirements.txt"},
		map[string]string{
			"pyproject.toml":       "[project]\nname='helper'\ndependencies=['click']\n[project.optional-dependencies]\ndocs=['flask>=3']\n",
			"web/requirements.txt": "Flask==3.0.0  # pinned\n",
		})
	insp.Unavailable = []Unavailable{{Path: "web/package.json", Reason: "fetch error"}}
	comps := buildInventory(indexInspection(insp))

	root := compByRoot(t, comps, "")
	got := map[string]string{}
	for _, d := range root.deps {
		got[d.Name] = d.Category
	}
	if got["click"] != "project.dependencies" || got["flask"] != "project.optional-dependencies.docs" {
		t.Errorf("python categories = %v; the docs extra must stay distinct from mandatory deps", got)
	}

	web := compByRoot(t, comps, "web")
	if m := web.manifest(kindNPM); m.state != StateUnavailable || m.reason != "fetch error" {
		t.Errorf("failed manifest lost its state: %+v", m)
	}
	if len(web.deps) != 1 || web.deps[0].Category != "requirements" || web.deps[0].Spec != "Flask==3.0.0" {
		t.Errorf("web deps = %+v, want the requirements.txt declaration with its specifier", web.deps)
	}
}

// Shuffling the listing, the read files and the failures yields the same
// normalized inventory, with original path case preserved.
func TestInventoryIsPermutationStable(t *testing.T) {
	base := Inspection{
		Paths: []string{"package.json", "UI/Package.json", "UI/index.html", "go.mod", "cmd/App/main.go", "packages/x/go.mod", "packages/x/x.go"},
		Files: []SourceFile{
			{Path: "package.json", Content: `{"dependencies":{"react":"18","express":"4"},"devDependencies":{"vite":"5"}}`},
			{Path: "UI/Package.json", Content: `{"dependencies":{"react":"18"}}`},
			{Path: "go.mod", Content: "module example.com/m\n\nrequire (\n\tgithub.com/spf13/cobra v1.8.0\n\tgolang.org/x/mod v0.17.0 // indirect\n)\n"},
			{Path: "cmd/App/main.go", Content: "package main\n"},
			{Path: "packages/x/go.mod", Content: "module example.com/x\n"},
		},
		Unavailable: []Unavailable{{Path: "packages/x/x.go", Reason: "not found"}},
	}
	want := buildInventory(indexInspection(base))
	if c := compByRoot(t, want, "UI"); !hasPath(c, "UI/index.html") {
		t.Errorf("original case lost: %v", c.paths)
	}

	r := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		shuffled := base
		shuffled.Paths = append([]string(nil), base.Paths...)
		shuffled.Files = append([]SourceFile(nil), base.Files...)
		r.Shuffle(len(shuffled.Paths), func(a, b int) { shuffled.Paths[a], shuffled.Paths[b] = shuffled.Paths[b], shuffled.Paths[a] })
		r.Shuffle(len(shuffled.Files), func(a, b int) { shuffled.Files[a], shuffled.Files[b] = shuffled.Files[b], shuffled.Files[a] })
		if got := buildInventory(indexInspection(shuffled)); !reflect.DeepEqual(got, want) {
			t.Fatalf("permutation %d changed the inventory", i)
		}
	}
}
