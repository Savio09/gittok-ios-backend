package crawler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/corpus"
)

// Task 11a: the real-repository acceptance corpus (see testdata/corpus/README.md
// for provenance and reference judgments). Every case asserts kind AND status.
// Counts are reported per set — resolved, correct, incorrect, inconclusive —
// with per-case outcomes; six selected repositories are not a measure of
// general accuracy.

type expect struct {
	kind   apimodel.RepoKind
	status VerdictStatus
}

func (e expect) String() string { return string(e.kind) + "/" + string(e.status) }

var (
	resolvedAs = func(k apimodel.RepoKind) expect { return expect{k, VerdictResolved} }
	abstains   = expect{apimodel.KindOther, VerdictInconclusive}
)

type corpusMutation struct {
	name    string
	apply   func(*Inspection)
	want    expect
	mention string // a limitation must name the degraded evidence
}

type corpusCase struct {
	file      string
	complete  expect // selection plus decisive extras
	pipeline  expect // the selection alone: what analyze sees
	component string // the component whose evidence decides the complete-enough verdict
	mutations []corpusMutation
}

// Mutations of a complete-enough inspection.
func unavailable(p string) func(*Inspection) {
	return func(i *Inspection) {
		for n := range i.Files {
			if i.Files[n].Path == p {
				i.Files = append(i.Files[:n], i.Files[n+1:]...)
				i.Unavailable = append(i.Unavailable, Unavailable{Path: p, Reason: "fetch error"})
				return
			}
		}
	}
}

func clipBefore(p, marker string) func(*Inspection) {
	return func(i *Inspection) {
		for n := range i.Files {
			if f := &i.Files[n]; f.Path == p {
				if at := strings.Index(f.Content, marker); at > 0 {
					f.Size, f.Content, f.Truncated = len(f.Content), f.Content[:at], true
				}
			}
		}
	}
}

func dropPath(p string) func(*Inspection) {
	return func(i *Inspection) {
		var paths []string
		for _, q := range i.Paths {
			if q != p {
				paths = append(paths, q)
			}
		}
		var files []SourceFile
		for _, f := range i.Files {
			if f.Path != p {
				files = append(files, f)
			}
		}
		i.Paths, i.Files = paths, files
	}
}

func truncatedListing(i *Inspection) { i.Scope.Truncated = true }

var corpusCases = []corpusCase{
	{
		file: "Savio09__plantly.json", complete: resolvedAs(apimodel.KindMobile), pipeline: resolvedAs(apimodel.KindMobile), component: "",
		mutations: []corpusMutation{
			{"root package.json unreadable", unavailable("package.json"), abstains, "package.json could not be read"},
			{"no README", dropPath("README.md"), resolvedAs(apimodel.KindMobile), ""},
		},
	},
	{
		file: "starship__starship.json", complete: resolvedAs(apimodel.KindCLI), pipeline: resolvedAs(apimodel.KindCLI), component: "",
		mutations: []corpusMutation{
			{"Cargo.toml unreadable", unavailable("Cargo.toml"), abstains, "Cargo.toml could not be read"},
			{"Cargo.toml clipped before [dependencies]", clipBefore("Cargo.toml", "[dependencies]"), abstains, "Cargo.toml was read only partially"},
			{"no README", dropPath("README.md"), resolvedAs(apimodel.KindCLI), ""},
		},
	},
	{
		file: "cli__cli.json", complete: resolvedAs(apimodel.KindCLI), pipeline: resolvedAs(apimodel.KindCLI), component: "",
		mutations: []corpusMutation{
			{"go.mod unreadable", unavailable("go.mod"), abstains, "go.mod could not be read"},
			{"truncated listing keeps the positive facts", truncatedListing, resolvedAs(apimodel.KindCLI), ""},
			{"no README", dropPath("README.md"), resolvedAs(apimodel.KindCLI), ""},
		},
	},
	{
		file: "excalidraw__excalidraw.json", complete: resolvedAs(apimodel.KindFrontend), pipeline: abstains, component: "excalidraw-app",
		mutations: []corpusMutation{
			{"the app's manifest unreadable", unavailable("excalidraw-app/package.json"), abstains, "excalidraw-app/package.json could not be read"},
			{"truncated listing", truncatedListing, abstains, "the listing is incomplete"},
		},
	},
	{
		file: "pocketbase__pocketbase.json", complete: resolvedAs(apimodel.KindLibrary), pipeline: abstains, component: "",
		mutations: []corpusMutation{
			{"cmd/serve.go unreadable", unavailable("cmd/serve.go"), abstains, "cmd/serve.go could not be read"},
			{"truncated listing", truncatedListing, abstains, "the listing does not cover cmd/<name>/"},
		},
	},
	{
		file: "Savio09__face-recognition.json", complete: abstains, pipeline: abstains, component: "",
	},
}

func loadSnapshot(t *testing.T, file string) corpus.Snapshot {
	t.Helper()
	s, err := corpus.Load(filepath.Join("testdata", "corpus", file))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func snapshotInspection(s corpus.Snapshot, paths []string) Inspection {
	return NewInspection(s.Language, Revision{Ref: s.ResolvedSHA, Resolved: true}, s.FileTree(), s.Fetched(paths))
}

// tally counts outcomes for one set, keeping correctness among resolved
// verdicts and the abstention count apart.
type tally struct{ resolved, correct, incorrect, inconclusive int }

func (c *tally) add(got Classification, want expect) {
	if got.Status == VerdictInconclusive {
		c.inconclusive++
	} else {
		c.resolved++
	}
	if got.Kind == want.kind && got.Status == want.status {
		if got.Status == VerdictResolved {
			c.correct++
		}
	} else if got.Status == VerdictResolved {
		c.incorrect++
	}
}

func (c tally) String() string {
	return fmt.Sprintf("resolved=%d correct=%d incorrect=%d inconclusive=%d", c.resolved, c.correct, c.incorrect, c.inconclusive)
}

func TestCorpusSnapshots(t *testing.T) {
	sha := regexp.MustCompile(`^[0-9a-f]{40}$`)
	var complete, pipeline, mutated tally

	for _, c := range corpusCases {
		s := loadSnapshot(t, c.file)
		t.Run(s.Repo, func(t *testing.T) {
			// Provenance and sanitization.
			if !sha.MatchString(s.ResolvedSHA) || s.RequestedRef == "" || s.CapturedAt == "" ||
				s.SourceURL != "https://github.com/"+s.Repo {
				t.Errorf("incomplete provenance: ref=%q sha=%q at=%q url=%q", s.RequestedRef, s.ResolvedSHA, s.CapturedAt, s.SourceURL)
			}
			if err := s.CheckSanitized(); err != nil {
				t.Error(err)
			}
			for p, why := range s.Extras {
				if strings.TrimSpace(why) == "" {
					t.Errorf("extra %s has no recorded reason", p)
				}
			}
			// Selected is historical capture provenance, not a demand that every
			// future selector repeat the old ranking. Keep its original judgments
			// below; TestG16CurrentSelectionReplaysCapturedEvidence exercises the
			// current selector against exactly the captured bytes it requests.
			if len(s.Selected) > s.ContentLimits.MaxFiles {
				t.Fatal("historical selection exceeds its recorded file budget")
			}

			full := Classify(snapshotInspection(s, s.AllRequested()))
			complete.add(full, c.complete)
			t.Logf("complete-enough: %s/%s (want %s)", full.Kind, full.Status, c.complete)
			if full.Kind != c.complete.kind || full.Status != c.complete.status {
				t.Errorf("complete-enough: got %s/%s, want %s\n%s", full.Kind, full.Status, c.complete, strings.Join(full.Evidence(), "\n"))
			}
			for _, o := range full.Observations {
				if o.Component != c.component {
					t.Errorf("deciding evidence came from component %q, want %q: %s", o.Component, c.component, o.Detail)
				}
			}

			sel2 := Classify(snapshotInspection(s, s.Selected))
			pipeline.add(sel2, c.pipeline)
			t.Logf("pipeline (selection only): %s/%s (want %s)", sel2.Kind, sel2.Status, c.pipeline)
			if sel2.Kind != c.pipeline.kind || sel2.Status != c.pipeline.status {
				t.Errorf("pipeline: got %s/%s, want %s\n%s", sel2.Kind, sel2.Status, c.pipeline, strings.Join(sel2.Evidence(), "\n"))
			}

			for _, m := range c.mutations {
				insp := snapshotInspection(s, s.AllRequested())
				m.apply(&insp)
				got := Classify(insp)
				mutated.add(got, m.want)
				t.Logf("mutation %q: %s/%s (want %s)", m.name, got.Kind, got.Status, m.want)
				if got.Kind != m.want.kind || got.Status != m.want.status {
					t.Errorf("mutation %q: got %s/%s, want %s", m.name, got.Kind, got.Status, m.want)
				}
				if m.mention != "" && !limitationsContain(got.Limitations, m.mention) {
					t.Errorf("mutation %q: no limitation names %q: %v", m.name, m.mention, got.Limitations)
				}
			}
		})
	}
	t.Logf("complete-enough set: %s", complete)
	t.Logf("pipeline set:        %s", pipeline)
	t.Logf("mutation set:        %s", mutated)
	if complete.incorrect+pipeline.incorrect+mutated.incorrect > 0 {
		t.Errorf("a resolved verdict was incorrect")
	}
}

// The October 30 historical packs, preserved verbatim as degradation cases.
// Their trees are abbreviated displays, so they are replayed as depth-limited
// listings; only Plantly, whose decisive root manifest is present, resolves.
func TestArchivedPacksDegradeHonestly(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "corpus", "archived-2026-10-30-packs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var packs []struct {
		Name, Language string
		Paths          []string
		Manifests      map[string]string
	}
	if err := json.Unmarshal(b, &packs); err != nil {
		t.Fatal(err)
	}
	want := map[string]expect{
		"Savio09__face-recognition": abstains,
		"Savio09__plantly":          resolvedAs(apimodel.KindMobile),
		"excalidraw__excalidraw":    abstains,
		"pocketbase__pocketbase":    abstains,
	}
	var archived tally
	for _, p := range packs {
		insp := InspectionFromSources(p.Language, p.Paths, p.Manifests)
		insp.Scope.DepthLimited = true // abbreviated display, never a complete-tree oracle
		got := Classify(insp)
		archived.add(got, want[p.Name])
		t.Logf("%s: %s/%s (want %s)", p.Name, got.Kind, got.Status, want[p.Name])
		if got.Kind != want[p.Name].kind || got.Status != want[p.Name].status {
			t.Errorf("%s: got %s/%s, want %s", p.Name, got.Kind, got.Status, want[p.Name])
		}
	}
	t.Logf("archived set: %s", archived)
}
