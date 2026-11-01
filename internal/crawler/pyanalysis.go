package crawler

import (
	"regexp"
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
		// Dynamic lists fields a build backend computes at build time; a
		// dynamic "dependencies" means the static list is not the real one.
		Dynamic []string `toml:"dynamic"`
	} `toml:"project"`
}

// dynamicEntryPoints reports whether [project] marks its scripts, GUI scripts or
// entry points dynamic, so the static tables are not authoritative.
func (p pyProject) dynamicEntryPoints() bool {
	if p.Project == nil {
		return false
	}
	for _, f := range p.Project.Dynamic {
		if f == "scripts" || f == "gui-scripts" || f == "entry-points" {
			return true
		}
	}
	return false
}

// dynamicDependencies reports whether [project] declares its dependencies (or
// optional dependencies) dynamic, so the static lists are not authoritative.
func (p pyProject) dynamicDependencies() bool {
	if p.Project == nil {
		return false
	}
	for _, f := range p.Project.Dynamic {
		if f == "dependencies" || f == "optional-dependencies" {
			return true
		}
	}
	return false
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

// pep508Name is the distribution-name grammar of PEP 508 (case-insensitive).
var pep508Name = regexp.MustCompile(`(?i)^([a-z0-9]|[a-z0-9][a-z0-9._-]*[a-z0-9])$`)

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
	s = strings.TrimSpace(s)
	if !pep508Name.MatchString(s) {
		return "" // not a distribution name (e.g. a local path), so not a requirement
	}
	s = strings.ToLower(s)
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

// parseRequirements reads a requirements.txt within a bounded, documented
// subset: one PEP 508 requirement per line, with extras, version specifiers,
// environment markers and direct "name @ url" references. Comments, blank lines
// and index/hash options are ignored. Anything that declares requirements the
// file itself does not contain — an include (-r/--requirement), a constraints
// file (-c/--constraint), or a bare URL/VCS requirement — is returned as
// unsupported, so a caller cannot read the inspected lines as the whole set.
// Includes are never fetched and nothing is installed or evaluated.
func parseRequirements(content string) (specs, unsupported []string) {
	// pip joins a line ending in a backslash with the next before parsing.
	content = strings.ReplaceAll(strings.ReplaceAll(content, "\\\r\n", ""), "\\\n", "")
	for line := range strings.SplitSeq(content, "\n") {
		line = stripRequirementComment(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch opt := fields[0]; {
		case opt == "-r" || opt == "--requirement" || opt == "-c" || opt == "--constraint" ||
			strings.HasPrefix(opt, "--requirement=") || strings.HasPrefix(opt, "--constraint="):
			unsupported = append(unsupported, "includes \""+line+"\", which was not followed")
			continue
		case opt == "-e" || opt == "--editable":
			if len(fields) > 1 && fields[1] != "." {
				unsupported = append(unsupported, "declares the editable requirement \""+line+"\", which was not resolved")
			}
			continue
		case strings.HasPrefix(opt, "-"):
			continue // index, hash and other options declare no requirement
		}
		if strings.Contains(line, "://") && !isDirectReference(line) {
			unsupported = append(unsupported, "declares the URL or VCS requirement \""+line+"\", whose distribution name was not resolved")
			continue
		}
		switch {
		case line == ".":
			// The project itself, whose own metadata is read separately.
		case pyDistName(line) != "":
			specs = append(specs, line)
		default:
			unsupported = append(unsupported, "declares \""+line+"\", which is not a named requirement (for example a local project path); its dependencies were not evaluated")
		}
	}
	return specs, unsupported
}

// stripRequirementComment removes a pip comment: a "#" at the start of a line
// or preceded by whitespace runs to the end of the line ("#" inside a URL, as in
// "#egg=", is not a comment).
func stripRequirementComment(line string) string {
	for i := 0; i < len(line); i++ {
		if line[i] == '#' && (i == 0 || line[i-1] == ' ' || line[i-1] == '\t') {
			return strings.TrimSpace(line[:i])
		}
	}
	return strings.TrimSpace(line)
}

// isDirectReference reports whether a requirement is a PEP 508 direct reference,
// "name[extras] @ url", whose distribution name precedes the URL.
func isDirectReference(line string) bool {
	at := strings.Index(line, "@")
	return at > 0 && !strings.Contains(line[:at], "://") && pyDistName(line[:at]) != ""
}

// pyRequirementLines returns the supported requirement specifiers in a
// requirements.txt (see parseRequirements).
func pyRequirementLines(content string) []string {
	specs, _ := parseRequirements(content)
	return specs
}
