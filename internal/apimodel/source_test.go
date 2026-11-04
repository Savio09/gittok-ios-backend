package apimodel

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSourceURLIsAdditive(t *testing.T) {
	old := `{"kind":"file","ref":"README.md","lines":"1-2"}`
	var s Source
	if err := json.Unmarshal([]byte(old), &s); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"url"`) {
		t.Fatal("optional URL changed legacy payload")
	}
	s.URL = "https://github.com/o/r/blob/0123456789abcdef0123456789abcdef01234567/README.md"
	b, err = json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var withURL Source
	if err := json.Unmarshal(b, &withURL); err != nil || withURL != s {
		t.Fatalf("URL did not round-trip: %+v %v", withURL, err)
	}
}

// additional_sources is optional and additive: legacy claims decode unchanged
// and an empty list is never serialized.
func TestClaimAdditionalSourcesAreAdditive(t *testing.T) {
	legacy := `{"label":"L","source":{"kind":"file","ref":"go.mod"},"explanation":"E","confidence":"confident"}`
	var c Claim
	if err := json.Unmarshal([]byte(legacy), &c); err != nil {
		t.Fatal(err)
	}
	if c.AdditionalSources != nil {
		t.Fatalf("legacy claim gained sources: %+v", c.AdditionalSources)
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "additional_sources") {
		t.Fatalf("empty additional sources serialized: %s", b)
	}
	c.AdditionalSources = []Source{{Kind: SourceAPI, Ref: "src/main.rs", URL: "https://api.github.com/repos/o/r/git/trees/0123456789abcdef0123456789abcdef01234567?recursive=1"}}
	b, _ = json.Marshal(c)
	var round Claim
	if err := json.Unmarshal(b, &round); err != nil || len(round.AdditionalSources) != 1 || round.AdditionalSources[0] != c.AdditionalSources[0] {
		t.Fatalf("additional sources did not round-trip: %s %+v %v", b, round, err)
	}
}
