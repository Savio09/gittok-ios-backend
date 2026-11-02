package preview

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
)

func TestBuildCapsAndInterleavesComponentsDeterministically(t *testing.T) {
	in := inputForTest()
	deps := map[string]string{}
	for i := 0; i < 100; i++ {
		deps[fmt.Sprintf("dep%d", i)] = "1"
	}
	body, _ := json.Marshal(map[string]any{"dependencies": deps})
	in.Inspection = crawler.InspectionFromSources("", []string{"a/package.json", "b/package.json"}, map[string]string{"a/package.json": string(body), "b/package.json": string(body)})
	in.Inspection.Revision = crawler.Revision{Ref: testSHA, Resolved: true}
	p := Build(in)
	claims := claimsIn(p)
	if len(claims) != 24 {
		t.Fatalf("unbounded output %d", len(claims))
	}
	counts := map[string]int{}
	for _, c := range claims {
		if c.Source.Kind == apimodel.SourceFile {
			counts[c.Source.Ref]++
		}
	}
	if counts["a/package.json"] < 10 || counts["b/package.json"] < 10 {
		t.Fatalf("component starvation %v", counts)
	}
	in.Inspection.Files[0], in.Inspection.Files[1] = in.Inspection.Files[1], in.Inspection.Files[0]
	if !reflect.DeepEqual(p, Build(in)) {
		t.Fatal("output depends on input order")
	}
	if !strings.Contains(p.Sections[len(p.Sections)-1].Body, "24 representative") {
		t.Fatal("missing output limit")
	}
}

func TestFileSourcesEscapePathsAndRejectForgedSpans(t *testing.T) {
	in := inputForTest()
	text := "line one\nline two\nline three\n"
	in.Inspection.Files = []crawler.SourceFile{{Path: "docs/a #?.md", Content: text}}
	w := crawler.Witness{Kind: crawler.WitnessSpan, Path: "docs/a #?.md", Start: 0, End: len("line one\nline two\n"), Quote: "line one\nline two\n"}
	s, q, err := fileSource(in, w)
	if err != nil || s.Lines != "1-2" || q != w.Quote || !strings.Contains(s.URL, "docs/a%20%23%3F.md#L1-L2") {
		t.Fatalf("bad escaped span: %+v %q %v", s, q, err)
	}
	w.End = len(text) + 1
	if _, _, err := fileSource(in, w); err == nil {
		t.Fatal("accepted out-of-bounds source")
	}
	w.Path = "../escape"
	if _, _, err := fileSource(in, w); err == nil {
		t.Fatal("accepted path traversal")
	}
}

func TestDuplicateFactsCannotConsumeTheClaimBudget(t *testing.T) {
	in := inputForTest()
	content := strings.Repeat("a==1\n", 100) + "b==2\nc==3\n"
	in.Inspection = crawler.InspectionFromSources("Python", []string{"requirements.txt"}, map[string]string{"requirements.txt": content})
	in.Inspection.Revision = crawler.Revision{Ref: testSHA, Resolved: true}
	p := Build(in)
	names := map[string]bool{}
	for _, c := range claimsIn(p) {
		names[c.Label] = true
	}
	if !names["Declares b"] || !names["Declares c"] {
		t.Fatalf("duplicates starved other facts: %v", names)
	}
}
