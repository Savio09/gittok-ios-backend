package crawler

import (
	"fmt"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

// Inspection must keep the declared version spelling. Parse's default version
// fixer canonicalizes it (v1.9 -> v1.9.0 and drops build metadata). Supply a fixer
// that checks the same supported semantic-version grammar without rewriting it.
// https://pkg.go.dev/golang.org/x/mod/modfile#Parse
func parseGoModSource(file, content string) (*modfile.File, error) {
	return modfile.Parse(file, []byte(content), func(_ string, version string) (string, error) {
		if module.CanonicalVersion(version) == "" {
			return "", fmt.Errorf("invalid module version %q", version)
		}
		return version, nil
	})
}
