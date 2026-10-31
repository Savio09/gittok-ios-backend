package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 7: modern Python entry points and the tooling-vs-package distinction.
func TestPythonEntryPointsAndTooling(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		files map[string]string
		want  apimodel.RepoKind
	}{
		{
			name:  "[project.scripts] is a CLI",
			paths: []string{"pyproject.toml", "src/tool.py"},
			files: map[string]string{"pyproject.toml": "[project]\nname = \"tool\"\nversion = \"0.1.0\"\n[project.scripts]\ntool = \"tool:main\"\n"},
			want:  apimodel.KindCLI,
		},
		{
			name:  "tooling-only pyproject is not a library",
			paths: []string{"pyproject.toml", "script.py"},
			files: map[string]string{"pyproject.toml": "[tool.ruff]\nline-length = 88\n"},
			want:  apimodel.KindOther,
		},
		{
			name:  "a [project] package with no command entry is a library",
			paths: []string{"pyproject.toml", "src/pkg/__init__.py"},
			files: map[string]string{"pyproject.toml": "[project]\nname = \"pkg\"\nversion = \"0.1.0\"\n"},
			want:  apimodel.KindLibrary,
		},
		{
			name:  "requirements-based flask app is a backend",
			paths: []string{"requirements.txt", "app.py"},
			files: map[string]string{"requirements.txt": "Flask==3.0.0\n"},
			want:  apimodel.KindBackend,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources("Python", c.paths, c.files))
			if got.Kind != c.want {
				t.Errorf("Kind = %q, want %q (evidence: %v)", got.Kind, c.want, got.Evidence())
			}
		})
	}
}

// A tooling-only pyproject abstains with an explanation rather than guessing.
func TestToolingOnlyPyprojectAbstains(t *testing.T) {
	got := Classify(InspectionFromSources("Python",
		[]string{"pyproject.toml"},
		map[string]string{"pyproject.toml": "[tool.black]\nline-length = 100\n"},
	))
	if got.Status != VerdictInconclusive {
		t.Errorf("Status = %q, want %q", got.Status, VerdictInconclusive)
	}
}
