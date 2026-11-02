package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestProductRoleExcludesTestOnlyServerCalls(t *testing.T) {
	for _, pkg := range []string{"main", "main_test"} {
		t.Run(pkg, func(t *testing.T) {
			insp := InspectionFromSources("Go", []string{"go.mod", "cmd/tool/main.go", "cmd/tool/main_test.go"}, map[string]string{
				"go.mod":                "module example.com/tool\n",
				"cmd/tool/main.go":      "package main\nfunc main(){}\n",
				"cmd/tool/main_test.go": "package " + pkg + "\nimport (\"net/http\"; \"testing\")\nfunc TestHTTP(t *testing.T){ http.ListenAndServe(\":8080\",nil) }\n",
			})
			got := Classify(insp)
			if got.Kind != apimodel.KindCLI || got.Status != VerdictResolved {
				t.Fatalf("test-only call changed product role: %s/%s: %v", got.Kind, got.Status, got.Evidence())
			}
			for _, o := range got.Observations {
				if o.Path == "cmd/tool/main_test.go" {
					t.Errorf("test source supports product role: %+v", o)
				}
			}
		})
	}
	runtime := Classify(InspectionFromSources("Go", []string{"go.mod", "cmd/tool/main.go"}, map[string]string{
		"go.mod": "module example.com/tool\n", "cmd/tool/main.go": "package main\nimport \"net/http\"\nfunc main(){http.ListenAndServe(\":8080\",nil)}\n",
	}))
	if runtime.Kind != apimodel.KindBackend || runtime.Status != VerdictResolved {
		t.Errorf("runtime call lost supported backend: %s/%s: %v", runtime.Kind, runtime.Status, runtime.Evidence())
	}
}
