package crawler

import (
	"fmt"
	"path"
	"reflect"
	"sort"
	"strings"
	"unicode/utf8"
)

type FactKind string

const (
	FactDependency FactKind = "dependency"
	FactScript     FactKind = "script"
	FactCommand    FactKind = "command"
	FactTarget     FactKind = "target"
	FactServerCall FactKind = "server-call"
	FactReadme     FactKind = "readme"
)

// Fact describes one declaration or construct, never an aggregate product role.
// Witness values are parsed values; only span quotes are exact source excerpts.
type Fact struct {
	Kind      FactKind
	Component string
	Name      string
	Spec      string
	Category  string
	Optional  bool
	Witness   Witness
}

// CollectFacts reuses component inventory and parser semantics independently of
// role resolution. Every field/construct witness is checked before publication.
func CollectFacts(insp Inspection) []Fact {
	st := indexInspection(insp)
	var out []Fact
	add := func(f Fact) {
		if !utf8.ValidString(f.Name + f.Spec + f.Component) {
			return
		}
		if validateWitness(st, f.Witness) == nil {
			out = append(out, f)
		}
	}
	for _, comp := range buildInventory(st) {
		for _, d := range comp.deps {
			w := dependencyWitness(st, d)
			add(Fact{Kind: FactDependency, Component: comp.root, Name: d.Name, Spec: d.Spec, Category: d.Category, Optional: d.Optional, Witness: w})
		}
		for _, m := range comp.manifests {
			if !m.readable() {
				continue
			}
			switch m.kind {
			case kindNPM:
				if p := parsePackageJSON(m.content); p != nil {
					for _, name := range sortedKeys(p.Scripts) {
						if strings.TrimSpace(p.Scripts[name]) != "" {
							add(Fact{Kind: FactScript, Component: comp.root, Name: name, Spec: p.Scripts[name], Category: "scripts", Witness: Witness{Kind: WitnessField, Path: m.path, Field: []string{"scripts", name}, Value: p.Scripts[name]}})
						}
					}
					if cmd, state := p.commandEntry(); state == yes {
						add(Fact{Kind: FactCommand, Component: comp.root, Name: strings.Join(cmd.field, "."), Spec: cmd.target, Category: "bin", Witness: Witness{Kind: WitnessField, Path: m.path, Field: cmd.field, Value: cmd.target}})
					}
				}
			case kindPyProject:
				p, _, ok := parsePyProject(m.content)
				if ok && p.Project != nil {
					for _, name := range sortedKeys(p.Project.Scripts) {
						target := p.Project.Scripts[name]
						if strings.TrimSpace(target) != "" {
							add(Fact{Kind: FactCommand, Component: comp.root, Name: name, Spec: target, Category: "project.scripts", Witness: Witness{Kind: WitnessField, Path: m.path, Field: []string{"project", "scripts", name}, Value: target}})
						}
					}
				}
			case kindCargo:
				if p, ok := parseCargo(m.content); ok {
					if len(p.Bin) > 0 {
						add(Fact{Kind: FactTarget, Component: comp.root, Name: "binary target table", Category: "bin", Witness: Witness{Kind: WitnessField, Path: m.path, Field: []string{"bin"}}})
					}
					if p.Lib != nil {
						add(Fact{Kind: FactTarget, Component: comp.root, Name: "library target table", Category: "lib", Witness: Witness{Kind: WitnessField, Path: m.path, Field: []string{"lib"}}})
					}
				}
			}
		}
		for _, p := range comp.paths {
			f, read := st.files[p]
			if !read {
				continue
			}
			if strings.HasSuffix(strings.ToLower(p), ".go") {
				g := st.goFile(p)
				if !g.test {
					for _, call := range g.serverCalls {
						if call.start < 0 || call.end <= call.start || call.end > len(f.Content) {
							continue
						}
						add(Fact{Kind: FactServerCall, Component: comp.root, Name: call.callee, Witness: Witness{Kind: WitnessSpan, Path: p, Start: call.start, End: call.end, Quote: f.Content[call.start:call.end], Construct: constructGoServerCall}})
					}
				}
			}
			if isReadme(p) {
				if w, ok := readmeWitness(f); ok {
					out = append(out, Fact{Kind: FactReadme, Component: comp.root, Name: "README passage", Witness: w})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		return factKey(a) < factKey(b)
	})
	return out
}

func factKey(f Fact) string {
	return string(f.Kind) + "\x00" + f.Component + "\x00" + f.Witness.Path + "\x00" + strings.Join(f.Witness.Field, "\x00") + fmt.Sprintf("\x00%09d\x00", f.Witness.Start) + f.Name
}

// ValidateFact rejects changes to kind, value, ownership or witness, even when a
// real path remains. Callers normally consume CollectFacts directly; this audit
// boundary also permits retained facts to be checked against another inspection.
func ValidateFact(insp Inspection, f Fact) error {
	for _, actual := range CollectFacts(insp) {
		if reflect.DeepEqual(actual, f) {
			return nil
		}
	}
	return fmt.Errorf("fact is not supported by this inspection")
}

func dependencyWitness(st *inspIndex, d depDecl) Witness {
	field := []string{d.Category, d.Name}
	if d.Category == "require (indirect)" {
		field = []string{"require", d.Name}
	}
	if d.Category == "project.dependencies" {
		field = []string{"project", "dependencies", d.Name}
	}
	if group, ok := strings.CutPrefix(d.Category, "project.optional-dependencies."); ok {
		field = []string{"project", "optional-dependencies", group, d.Name}
	}
	// Cargo tables carry version separately; presence of a path/workspace
	// dependency still establishes a declaration but never an invented version.
	if kind, _ := manifestKindOf(d.Source); (kind == kindCargo || kind == kindPubspec) && d.Spec != "" {
		if tree, err := st.parsedManifest(d.Source); err == nil {
			if got, ok := resolveField(tree, field); ok && got == "" {
				field = append(field, "version")
			}
		}
	}
	w := Witness{Kind: WitnessField, Path: d.Source, Field: field, Value: d.Spec}
	if span, supported, located := st.fieldLocation(d.Source, field); supported && located {
		w.Start, w.End, w.Quote = span[0], span[1], st.files[d.Source].Content[span[0]:span[1]]
	}
	return w
}

func isReadme(p string) bool {
	base := strings.ToLower(path.Base(p))
	return base == "readme" || strings.HasPrefix(base, "readme.")
}

func readmeWitness(f SourceFile) (Witness, bool) {
	if !utf8.ValidString(f.Content) {
		return Witness{}, false
	}
	offset := 0
	fenced := false
	for _, line := range strings.SplitAfter(f.Content, "\n") {
		text := strings.TrimSpace(line)
		if strings.HasPrefix(text, "```") || strings.HasPrefix(text, "~~~") {
			fenced = !fenced
		}
		if !fenced && text != "" && len(line) <= 1024 && !strings.HasPrefix(text, "#") && !strings.HasPrefix(text, "[!") && !strings.HasPrefix(text, "![") && !strings.HasPrefix(text, "<") && !strings.HasPrefix(text, "===") && !strings.HasPrefix(text, "---") && !strings.HasPrefix(text, "```") && !strings.HasPrefix(text, "~~~") {
			end := offset + len(strings.TrimRight(line, "\r\n"))
			// Quote a whole short paragraph, rather than a sentence fragment at
			// the author's hard-wrapped first line. Preserve intervening bytes.
			nextOffset := offset + len(line)
			for _, next := range strings.SplitAfter(f.Content[nextOffset:], "\n") {
				trimmed := strings.TrimSpace(next)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
					break
				}
				end = nextOffset + len(strings.TrimRight(next, "\r\n"))
				if end-offset > 1024 {
					return Witness{}, false
				}
				nextOffset += len(next)
			}
			if end > offset {
				return Witness{Kind: WitnessSpan, Path: f.Path, Start: offset, End: end, Quote: f.Content[offset:end], Construct: "readme.quote"}, true
			}
		}
		offset += len(line)
	}
	return Witness{}, false
}
