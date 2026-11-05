package apimodel

import (
	"encoding/json"
	"testing"
)

func TestEvidenceFieldsAreAdditiveAndOptional(t *testing.T) {
	for _, legacy := range []string{
		`{"label":"Declares react","source":{"ref":"package.json"},"explanation":"A declaration.","confidence":"confident","stance":"caution"}`,
		`{"key":"stars","label":"stars","value":"12","confidence":"confident"}`,
		`{"id":"o/r","owner":"o","name":"r","slug":"o/r","kind":"other","description":"","github_url":""}`,
	} {
		var value any
		var input map[string]json.RawMessage
		if err := json.Unmarshal([]byte(legacy), &input); err != nil {
			t.Fatal(err)
		}
		if _, ok := input["label"]; ok {
			if _, metric := input["key"]; metric {
				value = &Metric{}
			} else {
				value = &Claim{}
			}
		} else {
			value = &RepoPreview{}
		}
		if err := json.Unmarshal([]byte(legacy), value); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		var output map[string]json.RawMessage
		if err := json.Unmarshal(b, &output); err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"evidence_status", "kind_evidence_status", "evidence_mode"} {
			if _, present := output[key]; present {
				t.Errorf("legacy JSON gained %s: %s", key, b)
			}
		}
		for key, want := range input {
			if string(output[key]) != string(want) {
				t.Errorf("legacy field %s changed: %s -> %s", key, want, output[key])
			}
		}
	}
}

func TestEvidenceStatusVocabulary(t *testing.T) {
	for _, status := range []EvidenceStatus{"", EvidenceSupported, EvidenceInferred, EvidenceUnknown} {
		if !status.Valid() {
			t.Errorf("known evidence status %q rejected", status)
		}
	}
	if EvidenceStatus("future").Valid() {
		t.Fatal("unknown evidence status accepted")
	}
}

// Evidence support, execution and fit can vary independently. In particular,
// a supported static fact can still be a caution for the reader.
func TestEvidenceStatusDoesNotChangeExecutionOrFit(t *testing.T) {
	claim := Claim{Label: "Declares a build script", Source: Source{Ref: "package.json"}, Explanation: "A static declaration.", Confidence: Confident, EvidenceStatus: EvidenceSupported, Stance: Caution}
	p := RepoPreview{Kind: KindCLI, KindEvidenceStatus: EvidenceInferred, EvidenceMode: NotExecuted, Sections: []Section{{Kind: SectionSetup, Claims: []Claim{claim}, EvidenceMode: NotExecuted}}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var decoded RepoPreview
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	got := decoded.Sections[0].Claims[0]
	if got.EvidenceStatus != EvidenceSupported || got.Stance != Caution || got.Confidence != Confident || decoded.EvidenceMode != NotExecuted || decoded.KindEvidenceStatus != EvidenceInferred {
		t.Fatalf("evidence/runtime/fit collapsed: %+v", decoded)
	}
}
