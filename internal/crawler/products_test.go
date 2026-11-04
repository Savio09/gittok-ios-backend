package crawler

import (
	"reflect"
	"testing"
)

func TestProductsNameTheDecidingAndCandidateComponents(t *testing.T) {
	for name, tc := range map[string]struct {
		paths []string
		files map[string]string
		want  []string
	}{
		"resolved root": {[]string{"go.mod", "main.go"}, map[string]string{"go.mod": "module m\nrequire github.com/spf13/cobra v1.8.0\n", "main.go": "package main\nfunc main(){}\n"}, []string{""}},
		"comparison": {[]string{"web/package.json", "web/index.html", "lib/package.json"}, map[string]string{
			"web/package.json": `{"dependencies":{"react":"18"}}`, "lib/package.json": `{"name":"lib","exports":"./i.js"}`}, []string{"web", "lib"}},
		"competing roles": {[]string{"web/package.json", "web/index.html", "api/go.mod", "api/main.go"}, map[string]string{
			"web/package.json": `{"dependencies":{"react":"18"}}`, "api/go.mod": "module api\nrequire github.com/spf13/cobra v1.8.0\n", "api/main.go": "package main\nfunc main(){}\n"}, []string{"api", "web"}},
		"unresolved root": {[]string{"go.mod", "cmd/x/main.go"}, map[string]string{"go.mod": "module m\n"}, []string{""}},
		"no signal":       {[]string{"README.md"}, map[string]string{"README.md": "x"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			c := Classify(InspectionFromSources("", tc.paths, tc.files))
			if !reflect.DeepEqual(c.Products, tc.want) {
				t.Errorf("Products = %q, want %q (%s/%s: %v)", c.Products, tc.want, c.Kind, c.Status, c.Evidence())
			}
		})
	}
}

func TestCollectImportsUsesParsedSyntaxOnly(t *testing.T) {
	insp := InspectionFromSources("Go", []string{"go.mod", "main.go", "main_test.go", "tool/go.mod", "tool/x.go"}, map[string]string{
		"go.mod":       "module m\n",
		"main.go":      "package main\n// import \"github.com/fake/comment\"\nimport (\n\t\"fmt\"\n\t\"github.com/spf13/cobra\"\n)\nfunc main(){ fmt.Println(cobra.Command{}) }\n",
		"main_test.go": "package main\nimport \"github.com/stretchr/testify\"\n",
		"tool/go.mod":  "module tool\n",
		"tool/x.go":    "package x\nimport \"golang.org/x/sync\"\n",
	})
	got := CollectImports(insp)
	want := []Import{{"", "main.go", "fmt"}, {"", "main.go", "github.com/spf13/cobra"}, {"tool", "tool/x.go", "golang.org/x/sync"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("imports = %+v, want %+v", got, want)
	}
}

func TestSupportingPathIsAHintAtEveryLevel(t *testing.T) {
	for p, want := range map[string]bool{
		"script/api-host-gateway/gateway/main.go": true,
		".github/codeql/tests/x/go.mod":           true,
		"internal/testdata/a.json":                true,
		"acceptance/README.md":                    true,
		"pkg/server/tests/helper.go":              true,
		"cmd/gh/main.go":                          false,
		"go.mod":                                  false,
		"internal/scripting/eval.go":              false,
		"web/src/App.tsx":                         false,
		"buildkite.yml":                           false,
	} {
		if got := SupportingPath(p); got != want {
			t.Errorf("SupportingPath(%q) = %v, want %v", p, got, want)
		}
	}
}
