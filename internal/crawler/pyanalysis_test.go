package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 3c: Python dependencies are read from parsed declarations, so a project
// name, a comment or an unrelated tooling table is never mistaken for a
// dependency, while a real dependency still counts.

func TestPyDistNameExtractsDistribution(t *testing.T) {
	cases := map[string]string{
		"flask>=3.0":                 "flask",
		"uvicorn[standard]":          "uvicorn",
		"Django == 4.2":              "django",
		"pkg ; python_version>'3.8'": "pkg",
		"scikit-learn":               "scikit-learn",
		"flask_login":                "flask-login",
		"":                           "",
	}
	for spec, want := range cases {
		if got := pyDistName(spec); got != want {
			t.Errorf("pyDistName(%q) = %q, want %q", spec, got, want)
		}
	}
}

func TestPyProjectDependenciesVsName(t *testing.T) {
	// The project is *named* flask-utils but declares no dependencies.
	proj, hasProject, parsed := parsePyProject("[project]\nname = \"flask-utils\"\nversion = \"0.1.0\"\ndependencies = []\n")
	if !parsed || !hasProject {
		t.Fatalf("parse: parsed=%v hasProject=%v", parsed, hasProject)
	}
	if names := pyProjectDependencyNames(proj); len(names) != 0 {
		t.Errorf("project name leaked into dependencies: %v", names)
	}

	// A tooling-only file has no [project] table at all.
	_, hasProject, parsed = parsePyProject("[tool.ruff]\nline-length = 88\n")
	if !parsed {
		t.Fatal("tooling-only pyproject should still parse")
	}
	if hasProject {
		t.Errorf("a [tool.*]-only file reported a [project] table")
	}

	// A real dependency is extracted.
	proj, _, _ = parsePyProject("[project]\nname = \"svc\"\ndependencies = [\"flask>=3.0\", \"uvicorn[standard]\"]\n")
	got := pyProjectDependencyNames(proj)
	if len(got) != 2 || got[0] != "flask" || got[1] != "uvicorn" {
		t.Errorf("dependency names = %v, want [flask uvicorn]", got)
	}
}

func TestPyRequirementNames(t *testing.T) {
	reqs := "# runtime deps\nFlask==3.0.0\nrequests>=2  # http\n-r other.txt\n--hash=sha256:abc\n\ngunicorn\n"
	got := pyRequirementNames(reqs)
	want := []string{"flask", "requests", "gunicorn"}
	if len(got) != len(want) {
		t.Fatalf("requirement names = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("requirement[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// At the classifier level: the project named flask-utils is not a backend, and
// a real Flask dependency (in pyproject or requirements.txt) is.
func TestPythonDependencyDetection(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		files map[string]string
		want  apimodel.RepoKind
	}{
		{
			name:  "project named flask-utils is not a flask backend",
			paths: []string{"pyproject.toml"},
			files: map[string]string{"pyproject.toml": "[project]\nname = \"flask-utils\"\nversion = \"0.1.0\"\ndependencies = []\n"},
			want:  apimodel.KindLibrary,
		},
		{
			name:  "real flask dependency in pyproject is a backend",
			paths: []string{"pyproject.toml", "app/main.py"},
			files: map[string]string{"pyproject.toml": "[project]\nname = \"svc\"\ndependencies = [\"flask>=3.0\"]\n"},
			want:  apimodel.KindBackend,
		},
		{
			name:  "flask in requirements.txt is a backend",
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
