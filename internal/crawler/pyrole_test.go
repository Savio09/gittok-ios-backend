package crawler

import (
	"strings"
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

// Task 6 (R4): mandatory dependencies and optional groups stay separate through
// role inference, and dependency declarations the bounded parser does not read
// stay visible instead of becoming an empty list that justifies a negative.
func TestPythonDependencyCategories(t *testing.T) {
	cases := []struct {
		name    string
		paths   []string
		files   map[string]string
		want    apimodel.RepoKind
		status  VerdictStatus
		cites   string // a primary observation must contain this, when set
		mention string // a limitation must contain this, when set
	}{
		{
			name:  "Flask only in a docs extra is not a web application (R4)",
			paths: []string{"pyproject.toml", "src/helper/__init__.py"},
			files: map[string]string{"pyproject.toml": "[project]\nname='helper'\nversion='1.0'\n[project.optional-dependencies]\ndocs=['flask>=3']\n"},
			want:  apimodel.KindLibrary, status: VerdictResolved,
			mention: "only in the optional group project.optional-dependencies.docs",
		},
		{
			name:  "Flask in mandatory dependencies is a web framework dependency",
			paths: []string{"pyproject.toml", "app/main.py"},
			files: map[string]string{"pyproject.toml": "[project]\nname='svc'\ndependencies=['flask>=3.0']\n"},
			want:  apimodel.KindBackend, status: VerdictResolved,
			cites: `declares "flask>=3.0" in project.dependencies`,
		},
		{
			name:  "Flask pinned in requirements.txt",
			paths: []string{"requirements.txt", "app.py"},
			files: map[string]string{"requirements.txt": "Flask==3.0.0\n"},
			want:  apimodel.KindBackend, status: VerdictResolved,
			cites: `declares "Flask==3.0.0" in requirements`,
		},
		{
			name:  "dynamic dependencies cannot prove a package needs no web framework",
			paths: []string{"pyproject.toml"},
			files: map[string]string{"pyproject.toml": "[project]\nname='pkg'\ndynamic=['version','dependencies']\n"},
			want:  apimodel.KindOther, status: VerdictInconclusive,
			mention: "declares its dependencies dynamic",
		},
		{
			name:  "an unfollowed requirements include stays visible",
			paths: []string{"requirements.txt", "app.py"},
			files: map[string]string{"requirements.txt": "-r requirements/base.txt\n"},
			want:  apimodel.KindOther, status: VerdictInconclusive,
			mention: `includes "-r requirements/base.txt", which was not followed`,
		},
		{
			name:  "setup.py dependencies are not evaluated",
			paths: []string{"setup.py"},
			files: map[string]string{"setup.py": "from setuptools import setup\nsetup(name='core', install_requires=deps())\n"},
			want:  apimodel.KindOther, status: VerdictInconclusive,
			mention: "install_requires was not evaluated",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources("Python", c.paths, c.files))
			if got.Kind != c.want || got.Status != c.status {
				t.Fatalf("got %q/%q, want %q/%q: %v", got.Kind, got.Status, c.want, c.status, got.Evidence())
			}
			if c.cites != "" {
				found := false
				for _, o := range got.Observations {
					if strings.Contains(o.Detail, c.cites) {
						found = true
					}
				}
				if !found {
					t.Errorf("no primary observation contains %q: %+v", c.cites, got.Observations)
				}
			}
			if c.mention != "" && !limitationsContain(got.Limitations, c.mention) {
				t.Errorf("no limitation contains %q: %v", c.mention, got.Limitations)
			}
		})
	}
}
