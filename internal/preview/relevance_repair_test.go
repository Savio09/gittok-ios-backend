package preview

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestInconclusivePeersKeepDecisiveDependencies(t *testing.T) {
	for _, root := range []string{"a", "z"} {
		t.Run(root, func(t *testing.T) {
			deps := map[string]string{"react": "19"}
			for i := 0; i < 9; i++ {
				deps[fmt.Sprintf("a%d", i)] = "1"
			}
			body, _ := json.Marshal(map[string]any{"dependencies": deps})
			in := synthetic([]string{root + "/go.mod", root + "/main.go", "m/package.json", "m/index.html"}, map[string]string{
				root + "/go.mod":  "module example.com/app\nrequire github.com/spf13/cobra v1.8.0\n",
				root + "/main.go": "package main\nimport \"github.com/spf13/cobra\"\nfunc main() { _ = cobra.Command{} }\n",
				"m/package.json":  string(body),
			})
			p := Build(in)
			labels := measure(p).labels
			if !labels["Declares react"] || !labels["Declares github.com/spf13/cobra"] {
				t.Errorf("decisive candidate evidence was displaced: %v", depLabels(p))
			}
			if _, explained := kindProof(p); p.Kind != apimodel.KindOther || explained {
				t.Error("ranking peer evidence must not resolve the inconclusive repository")
			}
		})
	}
}

func TestRecordedExcalidrawRetainsItsFramework(t *testing.T) {
	in := corpusInput(t, "excalidraw__excalidraw.json")
	p := Build(in)
	var react bool
	for _, c := range claimsIn(p) {
		if err := ValidateClaim(in, c); err != nil {
			t.Fatalf("unsupported claim: %v", err)
		}
		react = react || (c.Label == "Declares react" && c.Source.Ref == "excalidraw-app/package.json")
	}
	if !react {
		t.Errorf("recorded product framework lost to ordinary declarations: %v", depLabels(p))
	}
}

func TestRootToolingDoesNotHideTheOnlyExecutable(t *testing.T) {
	for _, manifest := range []string{"go.mod", "tools/app/go.mod"} {
		t.Run(manifest, func(t *testing.T) {
			files := map[string]string{
				manifest:            "module example.com/app\nrequire (\n github.com/spf13/cobra v1.8.0\n github.com/spf13/pflag v1.0.5\n golang.org/x/term v0.20.0\n)\n",
				"tools/app/main.go": "package main\nimport (\"github.com/spf13/cobra\"; \"net/http\")\nfunc main() { _ = cobra.Command{}; http.ListenAndServe(\":8080\", nil) }\n",
				"README.md":         "# App\n\nThe product command.\n",
				"package.json":      `{"private":true,"devDependencies":{"prettier":"3"}}`,
			}
			p := Build(synthetic([]string{manifest, "tools/app/main.go", "README.md", "package.json"}, files))
			labels := measure(p).labels
			for _, want := range []string{"Contains net/http call", "Declares github.com/spf13/cobra", "Declares github.com/spf13/pflag", "Declares golang.org/x/term"} {
				if !labels[want] {
					t.Errorf("root tooling displaced %q: %v", want, depLabels(p))
				}
			}
		})
	}
}

func TestRealProductWithoutImportsStillDemotesHelpers(t *testing.T) {
	for _, pkg := range []string{"main", "lib"} {
		t.Run(pkg, func(t *testing.T) {
			in := synthetic([]string{"go.mod", "product.go", "tools/helper.go"}, map[string]string{
				"go.mod":          "module example.com/product\n",
				"product.go":      "package " + pkg + "\nfunc main() {}\n",
				"tools/helper.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":8080\", nil) }\n",
			})
			p := Build(in)
			if p.Kind == apimodel.KindOther {
				t.Fatal("fixture must have a real resolved root product without imports")
			}
			for _, c := range claimsIn(p) {
				if c.Label == "Contains net/http call" && strings.HasPrefix(c.Source.Ref, "tools/") {
					t.Error("a helper call became the product walkthrough")
				}
			}
		})
	}
}

func TestUnusedPeerSharesGoToProductBeforeDevelopment(t *testing.T) {
	for _, root := range []string{"a", "z"} {
		t.Run(root, func(t *testing.T) {
			mod := "module example.com/app\nrequire (\n github.com/spf13/cobra v1.8.0\n"
			code := "package main\nimport (\n \"github.com/spf13/cobra\"\n"
			for i := 0; i < 8; i++ {
				mod += fmt.Sprintf(" example.com/product%d v1.0.0\n", i)
				code += fmt.Sprintf(" _ \"example.com/product%d\"\n", i)
			}
			mod += ")\n"
			code += ")\nfunc main() { _ = cobra.Command{} }\n"
			in := synthetic([]string{"package.json", root + "/go.mod", root + "/main.go", "m/package.json", "m/index.html"}, map[string]string{
				"package.json":   `{"private":true,"devDependencies":{"eslint":"9","prettier":"3"}}`,
				root + "/go.mod": mod, root + "/main.go": code,
				"m/package.json": `{"dependencies":{"react":"19"}}`,
			})
			p := Build(in)
			var product, web, tooling int
			for _, c := range claimsIn(p) {
				if !strings.HasPrefix(c.Label, "Declares ") {
					continue
				}
				switch c.Source.Ref {
				case root + "/go.mod":
					product++
				case "m/package.json":
					web++
				case "package.json":
					tooling++
				}
			}
			if product != 7 || web != 1 || tooling != 0 {
				t.Errorf("unused product share was not redistributed: CLI=%d web=%d tooling=%d, %v", product, web, tooling, depLabels(p))
			}
			if !measure(p).labels["Declares github.com/spf13/cobra"] || !measure(p).labels["Declares react"] {
				t.Error("redistribution displaced a decisive peer dependency")
			}
		})
	}
}

func TestPeerReservationsRespectEffectiveCapacity(t *testing.T) {
	for _, tc := range []struct {
		name      string
		peers     int
		secondary bool
	}{
		{"three peers with secondary", 3, true},
		{"more peers than slots", 10, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var paths []string
			files := map[string]string{}
			if tc.secondary {
				paths = append(paths, "package.json")
				files["package.json"] = `{"private":true,"dependencies":{"format-helper":"1"}}`
			}
			for i := 0; i < tc.peers; i++ {
				root := fmt.Sprintf("peer%02d", i)
				deps := map[string]string{}
				for j := 0; j < 8; j++ {
					deps[fmt.Sprintf("a%d", j)] = "1"
				}
				pkg := map[string]any{"dependencies": deps}
				if i%2 == 0 {
					deps["react"] = "19"
					paths = append(paths, root+"/index.html")
				} else {
					pkg["bin"] = "./cli.js"
				}
				body, _ := json.Marshal(pkg)
				paths = append(paths, root+"/package.json")
				files[root+"/package.json"] = string(body)
			}
			in := synthetic(paths, files)
			p := Build(in)
			perPeer := map[string]int{}
			for _, c := range claimsIn(p) {
				if strings.HasPrefix(c.Label, "Declares ") && !strings.HasPrefix(c.Label, "Declares command ") && strings.HasPrefix(c.Source.Ref, "peer") {
					root, _, _ := strings.Cut(c.Source.Ref, "/")
					perPeer[root]++
				}
			}
			if tc.secondary {
				for i := 0; i < tc.peers; i++ {
					if perPeer[fmt.Sprintf("peer%02d", i)] != 2 {
						t.Errorf("effective six-product budget not shared equally: %v", perPeer)
					}
				}
				if !measure(p).labels["Declares format-helper"] {
					t.Error("secondary component lost its reserved opportunity")
				}
			} else if len(perPeer) != 8 {
				t.Errorf("eight slots did not represent eight peers: %v", perPeer)
			}
			if len(claimsIn(p)) > 24 || measure(p).deps > 8 || measure(p).low > 2 {
				t.Error("preview budget exceeded")
			}
			if p.Kind != apimodel.KindOther {
				t.Error("claim selection must not resolve the competing roles")
			}
		})
	}
}

func TestRootPackageEvidenceSurvivesATestDirectoryVerdict(t *testing.T) {
	in := synthetic([]string{"go.mod", "product.go", "tests/helper.go"}, map[string]string{
		"go.mod":          "module example.com/product\nrequire example.com/productdep v1.0.0\n",
		"product.go":      "package product\nfunc Product() {}\n",
		"tests/helper.go": "package main\nimport \"net/http\"\nfunc main() { http.ListenAndServe(\":8080\", nil) }\n",
	})
	for _, c := range claimsIn(Build(in)) {
		if c.Label == "Contains net/http call" && c.Source.Ref == "tests/helper.go" {
			t.Error("classifier-selected test evidence hid the actual root package")
		}
	}
}

func TestTestHelperImportsDoNotOutrankTheRootPackage(t *testing.T) {
	mod := "module example.com/product\nrequire (\n"
	helper := "package main\nimport (\n"
	for i := 0; i < 9; i++ {
		mod += fmt.Sprintf(" example.com/aproduct%d v1.0.0\n example.com/zhelper%d v1.0.0\n", i, i)
		helper += fmt.Sprintf(" _ \"example.com/zhelper%d\"\n", i)
	}
	mod += ")\n"
	helper += ")\nfunc main() {}\n"
	in := synthetic([]string{"go.mod", "product.go", "nested/tests/helper.go"}, map[string]string{
		"go.mod": mod, "product.go": "package product\nfunc Product() {}\n", "nested/tests/helper.go": helper,
	})
	p := Build(in)
	if !measure(p).labels["Declares example.com/aproduct0"] {
		t.Errorf("root declarations were displaced by helper imports: %v", depLabels(p))
	}
	for _, c := range claimsIn(p) {
		if strings.HasPrefix(c.Label, "Declares example.com/zhelper") {
			t.Errorf("helper-only import received product priority: %s", c.Label)
		}
	}
}
