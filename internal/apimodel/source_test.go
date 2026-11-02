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
