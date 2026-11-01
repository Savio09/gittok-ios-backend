package crawler

import (
	"strings"
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
	deps := func(content string) []depDecl {
		return manifestDeps(manifestRef{path: "pyproject.toml", kind: kindPyProject, state: StateRead, content: content})
	}
	// The project is *named* flask-utils but declares no dependencies.
	if got := deps("[project]\nname = \"flask-utils\"\nversion = \"0.1.0\"\ndependencies = []\n"); len(got) != 0 {
		t.Errorf("project name leaked into dependencies: %+v", got)
	}

	// A tooling-only file has no [project] table at all.
	_, hasProject, parsed := parsePyProject("[tool.ruff]\nline-length = 88\n")
	if !parsed || hasProject {
		t.Errorf("tooling-only pyproject: parsed=%v hasProject=%v, want parsed without a [project] table", parsed, hasProject)
	}

	// A real dependency is extracted, keeping its full specifier and table.
	got := deps("[project]\nname = \"svc\"\ndependencies = [\"flask>=3.0\", \"uvicorn[standard]\"]\n")
	if len(got) != 2 || got[0].Name != "flask" || got[0].Spec != "flask>=3.0" || got[1].Name != "uvicorn" ||
		got[0].Category != "project.dependencies" {
		t.Errorf("dependencies = %+v, want flask and uvicorn in project.dependencies", got)
	}
}

// Task 6: requirements.txt is read within a bounded subset; anything that
// declares requirements the file does not itself contain is reported, never
// silently treated as "no such requirement".
func TestParseRequirements(t *testing.T) {
	reqs := "# runtime deps\nFlask==3.0.0\nrequests>=2  # http\n-r other.txt\n--hash=sha256:abc\n\ngunicorn\n" +
		"uvicorn[standard] ; python_version >= '3.9'\nmypkg @ https://example.com/mypkg-1.0.whl\n" +
		"git+https://github.com/acme/tool.git#egg=tool\n-c constraints.txt\n-e .\n"
	specs, unsupported := parseRequirements(reqs)

	var names []string
	for _, s := range specs {
		names = append(names, pyDistName(s))
	}
	if want := []string{"flask", "requests", "gunicorn", "uvicorn", "mypkg"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("names = %v, want %v", names, want)
	}
	for _, want := range []string{"-r other.txt", "git+https://github.com/acme/tool.git", "-c constraints.txt"} {
		found := false
		for _, u := range unsupported {
			if strings.Contains(u, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("unsupported = %v, want an entry for %q", unsupported, want)
		}
	}
	if len(unsupported) != 3 {
		t.Errorf("unsupported = %v, want exactly the include, the VCS line and the constraints file", unsupported)
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
