package crawler

import (
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

// Slice 6: Rust targets follow Cargo's conventions, including the implicit
// src/main.rs, src/bin/ and src/lib.rs targets, so an ordinary binary or library
// crate is classified without an explicit [[bin]] or [lib] table.
func TestRustConventionalTargets(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		files map[string]string
		want  apimodel.RepoKind
	}{
		{
			name:  "implicit binary with clap is a CLI (no [[bin]] needed)",
			paths: []string{"Cargo.toml", "src/main.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"tool\"\nversion = \"0.1.0\"\n[dependencies]\nclap = \"4\"\n"},
			want:  apimodel.KindCLI,
		},
		{
			name:  "implicit library is a library (no [lib] needed)",
			paths: []string{"Cargo.toml", "src/lib.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"math\"\nversion = \"0.1.0\"\n"},
			want:  apimodel.KindLibrary,
		},
		{
			name:  "implicit binary with axum is a backend",
			paths: []string{"Cargo.toml", "src/main.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"api\"\n[dependencies]\naxum = \"0.7\"\n"},
			want:  apimodel.KindBackend,
		},
		{
			name:  "mixed lib + bin is classified by the binary without absence claims",
			paths: []string{"Cargo.toml", "src/lib.rs", "src/main.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"both\"\n[dependencies]\nclap = \"4\"\n"},
			want:  apimodel.KindCLI,
		},
		{
			name:  "autobins = false disables the implicit binary target",
			paths: []string{"Cargo.toml", "src/main.rs", "src/lib.rs"},
			files: map[string]string{"Cargo.toml": "[package]\nname = \"libonly\"\nautobins = false\n"},
			want:  apimodel.KindLibrary,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Classify(InspectionFromSources("Rust", c.paths, c.files))
			if got.Kind != c.want {
				t.Errorf("Kind = %q, want %q (evidence: %v)", got.Kind, c.want, got.Evidence())
			}
		})
	}
}

// A CLI-parser dependency without any binary target does not make a crate a CLI.
func TestRustClapWithoutBinaryIsNotCLI(t *testing.T) {
	insp := InspectionFromSources("Rust",
		[]string{"Cargo.toml", "src/lib.rs"},
		map[string]string{"Cargo.toml": "[package]\nname = \"helper\"\n[dependencies]\nclap = \"4\"\n"},
	)
	if got := Classify(insp); got.Kind != apimodel.KindLibrary {
		t.Errorf("Kind = %q, want %q (clap in a library is not a CLI)", got.Kind, apimodel.KindLibrary)
	}
}
