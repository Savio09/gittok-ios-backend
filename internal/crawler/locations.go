package crawler

import (
	"strings"
)

// This file locates manifest fields in the bytes the analyzer consumed, so a
// field witness can cite the exact declaration (and its lines) rather than the
// whole file. A location is only ever derived from the same parser that
// established the field's value; it is never found by searching the text.
//
// Supported locators:
//   - go.mod requirements, from x/mod's own syntax positions ("require", path).
//
// Other fields keep whole-file citations: a span claimed on them is rejected.

// fieldLocation reports where field's effective declaration lies in p.
// supported is false when no verified locator covers this field; located is
// false when a supported field has no declaration wholly inside the consumed
// bytes (for example, one cut by the read limit).
func (st *inspIndex) fieldLocation(p string, field []string) (span [2]int, supported, located bool) {
	if !locatableField(p, field) {
		return span, false, false
	}
	locs := st.fieldLocations(p)
	span, located = locs[locationKey(field)]
	return span, true, located
}

func locatableField(p string, field []string) bool {
	kind, _ := manifestKindOf(p)
	return kind == kindGoMod && len(field) == 2 && field[0] == "require"
}

func locationKey(field []string) string { return strings.Join(field, "\x00") }

// fieldLocations parses p once per inspection and caches every location.
func (st *inspIndex) fieldLocations(p string) map[string][2]int {
	if locs, ok := st.locationCache[p]; ok {
		return locs
	}
	var locs map[string][2]int
	if f, ok := st.files[p]; ok {
		if kind, _ := manifestKindOf(p); kind == kindGoMod {
			locs = goModLocations(p, f.Content, !f.Truncated)
		}
	}
	if st.locationCache == nil {
		st.locationCache = make(map[string]map[string][2]int)
	}
	st.locationCache[p] = locs
	return locs
}

// goModLocations maps each effective requirement to its declaration's bytes:
// the requirement line from x/mod's syntax positions (including the "require"
// keyword on a single-line form, and any quotes), extended over its suffix
// comment when that comment marks it indirect. As in the parsed field tree, a
// repeated path is represented by its last declaration.
//
// When the file was clipped, only declarations whose line ends before the last
// newline in the consumed bytes are located: the final line may have been cut
// mid-token ("v1.2" of "v1.2.3"), so it cannot support a declaration.
func goModLocations(p, content string, complete bool) map[string][2]int {
	f, err := parseGoModSource(p, content)
	if err != nil {
		return nil
	}
	limit := len(content)
	if !complete {
		limit = strings.LastIndexByte(content, '\n')
	}
	out := make(map[string][2]int, len(f.Require))
	for _, r := range f.Require {
		key := locationKey([]string{"require", r.Mod.Path})
		if r.Syntax == nil {
			delete(out, key)
			continue
		}
		start, end := r.Syntax.Start.Byte, r.Syntax.End.Byte
		if r.Indirect {
			for _, c := range r.Syntax.Suffix {
				if e := c.Start.Byte + len(strings.TrimRight(c.Token, "\r")); e > end {
					end = e
				}
			}
		}
		if start < 0 || end <= start || end > limit {
			delete(out, key)
			continue
		}
		out[key] = [2]int{start, end}
	}
	return out
}
