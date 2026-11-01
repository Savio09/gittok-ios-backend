package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

func TestFrameworkRequirementsUseCanonicalModuleIdentities(t *testing.T) {
	for _, tc := range []struct {
		module, version string
		want            apimodel.RepoKind
		framework       bool
	}{
		{"github.com/gin-gonic/gin", "v1.9.1", apimodel.KindBackend, true},
		{"github.com/labstack/echo/v4", "v4.12.0", apimodel.KindBackend, true},
		{"github.com/go-chi/chi/v5", "v5.0.0", apimodel.KindBackend, true},
		{"github.com/gofiber/fiber/v2", "v2.0.0", apimodel.KindBackend, true},
		{"github.com/gorilla/mux", "v1.8.0", apimodel.KindBackend, true},
		{"github.com/spf13/cobra", "v1.8.0", apimodel.KindCLI, true},
		{"github.com/urfave/cli/v3", "v3.0.0", apimodel.KindCLI, true},
		{"gopkg.in/urfave/cli.v1", "v1.20.0", apimodel.KindCLI, true},
		{"gopkg.in/alecthomas/kingpin.v2", "v2.4.0", apimodel.KindCLI, true},
		{"example.com/gin-gonic/gin-helper", "v1.0.0", apimodel.KindCLI, false},
		{"example.com/github.com/gin-gonic/gin", "v1.0.0", apimodel.KindCLI, false},
		{"github.com/gin-gonic/gin/adapter", "v0.1.0", apimodel.KindCLI, false},
		{"github.com/spf13/cobra-helper", "v1.0.0", apimodel.KindCLI, false},
	} {
		t.Run(tc.module, func(t *testing.T) {
			got := Classify(InspectionFromSources("Go", []string{"go.mod", "main.go"}, map[string]string{
				"go.mod":  "module example.com/tool\n\nrequire " + tc.module + " " + tc.version + "\n",
				"main.go": "package main\nfunc main(){}\n",
			}))
			if got.Kind != tc.want || got.Status != VerdictResolved {
				t.Errorf("got %s/%s, want %s/resolved: %v", got.Kind, got.Status, tc.want, got.Evidence())
			}
			cited := false
			for _, o := range got.Observations {
				if o.Path == "go.mod" && len(o.Witness.Field) == 2 && o.Witness.Field[0] == "require" && o.Witness.Field[1] == tc.module {
					cited = true
					if o.Witness.Value != tc.version {
						t.Errorf("declared version lost: %+v", o)
					}
				}
			}
			if cited != tc.framework {
				t.Errorf("framework cited=%t, want %t: %v", cited, tc.framework, got.Evidence())
			}
		})
	}
}
