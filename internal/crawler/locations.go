package crawler

import (
	"encoding/json"
	"strconv"
	"strings"
)

// This file locates manifest fields in the bytes the analyzer consumed, so a
// field witness can cite the exact declaration (and its lines) rather than the
// whole file. A location is only ever derived from the same parser that
// established the field's value; it is never found by searching the text.
//
// Supported locators:
//   - go.mod requirements, from x/mod's own syntax positions ("require", path);
//   - package.json object members, from encoding/json's token stream.
//
// Other fields keep whole-file citations: a span claimed on them is rejected.

// fieldLocation reports where field's effective declaration lies in p.
// supported is false when no verified locator covers this field (including a
// locator that could not process the file); located is false when a supported
// field has no declaration wholly inside the consumed bytes (for example, one
// cut by the read limit).
func (st *inspIndex) fieldLocation(p string, field []string) (span [2]int, supported, located bool) {
	if !locatableField(p, field) {
		return span, false, false
	}
	locs := st.fieldLocations(p)
	if locs == nil {
		return span, false, false
	}
	span, located = locs[locationKey(field)]
	return span, true, located
}

func locatableField(p string, field []string) bool {
	switch kind, _ := manifestKindOf(p); kind {
	case kindGoMod:
		return len(field) == 2 && field[0] == "require"
	case kindNPM:
		return len(field) > 0
	}
	return false
}

// locationKey encodes a field path structurally: each segment is a quoted
// string, so no segment content (a NUL, a separator) can alias another path.
func locationKey(field []string) string {
	parts := make([]string, len(field))
	for i, f := range field {
		parts[i] = strconv.Quote(f)
	}
	return strings.Join(parts, ",")
}

// descends reports whether path lies strictly below parent.
func descends(path, parent []string) bool {
	if len(path) <= len(parent) {
		return false
	}
	for i := range parent {
		if path[i] != parent[i] {
			return false
		}
	}
	return true
}

// locate returns w with the verified location of its field attached, when one
// exists. The witness's value and semantics are unchanged.
func (st *inspIndex) locate(w Witness) Witness {
	if w.Kind != WitnessField {
		return w
	}
	if span, supported, located := st.fieldLocation(w.Path, w.Field); supported && located {
		w.Start, w.End, w.Quote = span[0], span[1], st.files[w.Path].Content[span[0]:span[1]]
	}
	return w
}

// fieldLocations parses p once per inspection and caches every location. A
// nil result means no locator processed the file.
func (st *inspIndex) fieldLocations(p string) map[string][2]int {
	if locs, ok := st.locationCache[p]; ok {
		return locs
	}
	var locs map[string][2]int
	if f, ok := st.files[p]; ok {
		switch kind, _ := manifestKindOf(p); kind {
		case kindGoMod:
			locs = goModLocations(p, f.Content, !f.Truncated)
		case kindNPM:
			locs = jsonLocations(f.Content)
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

// jsonLocations maps every object member's key path to the member's bytes,
// from the opening quote of its key to the end of its value. It walks
// encoding/json's token stream, so keys are decoded (escapes included) exactly
// as the parser that established the field's value decodes them, and a
// repeated key is represented by its last member, as in the parsed tree.
// Between tokens only JSON whitespace and separators are skipped; a key's
// position is never found by searching for its text. Members inside arrays are
// not addressable fields and are not recorded. Any input the walk does not
// accept as a single complete JSON value yields nil.
func jsonLocations(content string) map[string][2]int {
	dec := json.NewDecoder(strings.NewReader(content))
	out := map[string][2]int{}
	paths := map[string][]string{} // key -> its field path, for structural purges
	var value func(path []string, record bool) (int, bool)
	value = func(path []string, record bool) (int, bool) {
		tok, err := dec.Token()
		if err != nil {
			return 0, false
		}
		switch tok {
		case json.Delim('{'):
			for dec.More() {
				start := int(dec.InputOffset())
				for start < len(content) && strings.IndexByte(" \t\r\n,", content[start]) >= 0 {
					start++
				}
				if start >= len(content) || content[start] != '"' {
					return 0, false
				}
				key, err := dec.Token()
				name, isKey := key.(string)
				if err != nil || !isKey {
					return 0, false
				}
				member := append(append([]string(nil), path...), name)
				if record {
					// A repeated key replaces the earlier member and everything
					// recorded beneath it, as decoding into the tree does. Purge
					// before descending, so the effective member's children stay.
					for k, p := range paths {
						if descends(p, member) {
							delete(out, k)
							delete(paths, k)
						}
					}
				}
				end, ok := value(member, record)
				if !ok {
					return 0, false
				}
				if record {
					out[locationKey(member)] = [2]int{start, end}
					paths[locationKey(member)] = member
				}
			}
			if end, err := dec.Token(); err != nil || end != json.Delim('}') {
				return 0, false
			}
		case json.Delim('['):
			for dec.More() {
				if _, ok := value(path, false); !ok {
					return 0, false
				}
			}
			if end, err := dec.Token(); err != nil || end != json.Delim(']') {
				return 0, false
			}
		}
		return int(dec.InputOffset()), true
	}
	if _, ok := value(nil, true); !ok {
		return nil
	}
	if _, err := dec.Token(); err == nil {
		return nil // trailing data: not a single JSON value
	}
	return out
}
