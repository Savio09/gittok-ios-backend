package crawler

import (
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// This file holds the Python declaration parsing the classifier relies on, so a
// dependency is read from a parsed pyproject.toml or requirements.txt rather
// than from a substring. The classification review (Slice 3c) showed the old
// substring match turning the project *name* "flask-utils" into a Flask
// dependency; parsing the actual [project] table closes that. setup.py is never
// executed or scraped — its dependencies are dynamic and remain uninspected.

// pyProject is the slice of pyproject.toml the classifier reads. Project is a
// pointer so its absence (a tooling-only file with just [tool.*]) is
// distinguishable from an empty [project] table.
type pyProject struct {
	Project *struct {
		Name                 string              `toml:"name"`
		Dependencies         []string            `toml:"dependencies"`
		OptionalDependencies map[string][]string `toml:"optional-dependencies"`
		Scripts              map[string]string   `toml:"scripts"`
		GUIScripts           map[string]string   `toml:"gui-scripts"`
	} `toml:"project"`
}

// parsePyProject parses pyproject.toml. parsed is false when the TOML is
// malformed; hasProject is true only when a [project] table is actually present,
// so a file carrying only tool configuration is not mistaken for a package.
func parsePyProject(content string) (proj pyProject, hasProject, parsed bool) {
	if err := toml.Unmarshal([]byte(content), &proj); err != nil {
		return pyProject{}, false, false
	}
	return proj, proj.Project != nil, true
}

// pyDistName extracts the distribution name from a PEP 508 dependency specifier
// (e.g. "flask>=3.0", "uvicorn[standard]", "pkg ; python_version>'3.8'") and
// normalises it (lower-case, runs of -_. collapsed to a single -) so names
// compare per PEP 503. It returns "" for a spec with no leading name.
func pyDistName(spec string) string {
	s := strings.TrimSpace(spec)
	i := strings.IndexFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '<', '>', '=', '!', '~', ';', '[', '(', '@', ',':
			return true
		}
		return false
	})
	if i >= 0 {
		s = s[:i]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	// PEP 503 normalisation: collapse runs of -, _ and . to a single -.
	var b strings.Builder
	prevSep := false
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' {
			if !prevSep {
				b.WriteByte('-')
			}
			prevSep = true
			continue
		}
		b.WriteRune(r)
		prevSep = false
	}
	return strings.Trim(b.String(), "-")
}

// pyProjectDependencyNames returns the normalised distribution names declared in
// a parsed pyproject's [project].dependencies and [project.optional-dependencies].
func pyProjectDependencyNames(p pyProject) []string {
	if p.Project == nil {
		return nil
	}
	var names []string
	for _, spec := range p.Project.Dependencies {
		if n := pyDistName(spec); n != "" {
			names = append(names, n)
		}
	}
	for _, group := range p.Project.OptionalDependencies {
		for _, spec := range group {
			if n := pyDistName(spec); n != "" {
				names = append(names, n)
			}
		}
	}
	return names
}

// pyRequirementLines returns the requirement specifiers in a requirements.txt.
// Comments, blank lines, option lines (-r, -e, --hash) and inline comments are
// ignored; remote includes are never fetched.
func pyRequirementLines(content string) []string {
	var lines []string
	for line := range strings.SplitSeq(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") {
			continue
		}
		if i := strings.Index(line, " #"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if pyDistName(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// pyRequirementNames returns the normalised distribution names declared in a
// requirements.txt; environment markers and extras are stripped by pyDistName.
func pyRequirementNames(content string) []string {
	var names []string
	for _, line := range pyRequirementLines(content) {
		names = append(names, pyDistName(line))
	}
	return names
}
