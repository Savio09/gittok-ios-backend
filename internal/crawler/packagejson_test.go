package crawler

import "testing"

// Slice 3b: a parsed package.json keeps each dependency's category, so a
// build-time dev dependency is never mistaken for a runtime one.
func TestPackageJSONRetainsDependencyCategory(t *testing.T) {
	p := parsePackageJSON(`{
		"dependencies": {"react": "^18.2.0"},
		"devDependencies": {"vite": "^5.0.0"},
		"peerDependencies": {"react-dom": "^18.0.0"}
	}`)
	if p == nil {
		t.Fatal("package.json did not parse")
	}

	cases := []struct {
		name    string
		wantVer string
		wantCat depCategory
	}{
		{"react", "^18.2.0", depRuntime},
		{"vite", "^5.0.0", depDev},
		{"react-dom", "^18.0.0", depPeer},
	}
	for _, c := range cases {
		v, cat, ok := p.lookupDep(c.name)
		if !ok {
			t.Errorf("%s: not found", c.name)
			continue
		}
		if v != c.wantVer || cat != c.wantCat {
			t.Errorf("%s: got (%q, %q), want (%q, %q)", c.name, v, cat, c.wantVer, c.wantCat)
		}
	}

	if _, _, ok := p.lookupDep("express"); ok {
		t.Errorf("absent dependency reported as present")
	}
}
