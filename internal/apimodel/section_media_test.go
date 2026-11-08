package apimodel

import (
	"encoding/json"
	"testing"
)

func TestSectionMediaPreservesOrderedIndependentArtifacts(t *testing.T) {
	raw := []byte(`{"id":"o/r","owner":"o","name":"r","slug":"o/r","kind":"frontend","description":"D","github_url":"https://github.com/o/r","evidence_mode":"observed_running","sections":[{"kind":"how","title":"First screen","media":{"kind":"image","url":"https://example.com/one.png","evidence_mode":"observed_running"}},{"kind":"how","title":"Second screen","media":{"kind":"image","url":"https://example.com/two.png"}},{"kind":"how","title":"Source explanation","body":"Inspected source"}]}`)
	var preview RepoPreview
	if err := json.Unmarshal(raw, &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Sections) != 3 {
		t.Fatalf("sections = %d", len(preview.Sections))
	}
	first, second, third := preview.Sections[0], preview.Sections[1], preview.Sections[2]
	if first.Media == nil || first.Media.URL != "https://example.com/one.png" || first.Media.EvidenceMode != ObservedRunning {
		t.Fatalf("first artifact lost: %#v", first)
	}
	if second.Media == nil || second.Media.URL != "https://example.com/two.png" || second.Media.EvidenceMode != "" {
		t.Fatalf("second artifact lost or promoted: %#v", second)
	}
	if third.Media != nil || third.EvidenceMode != "" {
		t.Fatal("source explanation inherited capture provenance")
	}
	encoded, err := json.Marshal(preview)
	if err != nil {
		t.Fatal(err)
	}
	var restored RepoPreview
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Sections[1].Media == nil || restored.Sections[1].Media.URL != second.Media.URL {
		t.Fatal("artifact order or payload lost")
	}
}
