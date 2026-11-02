package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/corpus"
	"github.com/Savio09/gittok-ios-backend/internal/crawler"
	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

// Task 11b: the real-repository corpus driven through the whole bounded
// analysis path — metadata, tree, canonical selection, bounded contents,
// inspection, validated observations, resolution and pack rendering — against
// a mock GitHub that serves each snapshot exactly as captured. Classifier-only
// fixtures cannot show that acquisition preserves the evidence; this does.

// snapshotServer serves one snapshot. It records every contents request and the
// ref each request used; files in drop 404 as failed reads; pad inserts filler
// before a marker in a file so the client's own per-file cap clips it; and
// onContents, when set, sees each contents path as its request arrives.
type snapshotServer struct {
	s          corpus.Snapshot
	drop       map[string]bool
	pad        map[string]string // path -> marker to insert 70 KiB of filler before
	onContents func(path string, r *http.Request)

	mu        sync.Mutex
	requested []string
	refs      map[string]bool
}

func (m *snapshotServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	m.refs = map[string]bool{}
	owner, name, _ := strings.Cut(m.s.Repo, "/")
	prefix := "/repos/" + owner + "/" + name
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, prefix)
		switch {
		case !ok:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case rest == "":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": name, "full_name": m.s.Repo, "owner": map[string]any{"login": owner},
				"language": m.s.Language, "default_branch": m.s.RequestedRef, "archived": m.s.Archived,
				"html_url": m.s.SourceURL,
			})
		case strings.HasPrefix(rest, "/git/trees/"):
			m.noteRef(strings.TrimPrefix(rest, "/git/trees/"))
			tree := make([]map[string]any, 0, len(m.s.Tree.Entries))
			for _, e := range m.s.Tree.Entries {
				tree = append(tree, map[string]any{"path": e.Path, "type": e.Type, "size": e.Size})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"truncated": m.s.Tree.Truncated, "tree": tree})
		case strings.HasPrefix(rest, "/contents/"):
			p := strings.TrimPrefix(rest, "/contents/")
			m.mu.Lock()
			m.requested = append(m.requested, p)
			m.mu.Unlock()
			m.noteRef(r.URL.Query().Get("ref"))
			if m.onContents != nil {
				m.onContents(p, r)
			}
			f, captured := m.s.Files[p]
			if !captured || m.drop[p] {
				http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
				return
			}
			content := f.Content
			if marker, ok := m.pad[p]; ok {
				filler := "# " + strings.Repeat("x", 70*1024) + "\n"
				content = strings.Replace(content, marker, filler+marker, 1)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"type": "file", "encoding": "base64", "size": len(content),
				"content": base64.StdEncoding.EncodeToString([]byte(content)),
			})
		default:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func (m *snapshotServer) noteRef(ref string) {
	m.mu.Lock()
	m.refs[ref] = true
	m.mu.Unlock()
}

func loadCorpus(t *testing.T, file string) corpus.Snapshot {
	t.Helper()
	s, err := corpus.Load(filepath.Join("..", "..", "internal", "crawler", "testdata", "corpus", file))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func runPipeline(t *testing.T, m *snapshotServer) (analysis, error) {
	t.Helper()
	srv := m.start(t)
	owner, name, _ := strings.Cut(m.s.Repo, "/")
	c := githubclient.New("t", githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
	return analyze(context.Background(), c, owner, name)
}

type pipelineCase struct {
	file     string
	kind     apimodel.RepoKind
	status   crawler.VerdictStatus
	mentions []string // limitations an abstention must carry, naming what was not fetched
}

var pipelineCases = []pipelineCase{
	{file: "Savio09__plantly.json", kind: apimodel.KindMobile, status: crawler.VerdictResolved},
	{file: "starship__starship.json", kind: apimodel.KindCLI, status: crawler.VerdictResolved},
	{file: "cli__cli.json", kind: apimodel.KindCLI, status: crawler.VerdictResolved},
	{file: "excalidraw__excalidraw.json", kind: apimodel.KindOther, status: crawler.VerdictInconclusive,
		mentions: []string{"packages/excalidraw/package.json is listed but its content was not fetched"}},
	{file: "pocketbase__pocketbase.json", kind: apimodel.KindOther, status: crawler.VerdictInconclusive,
		mentions: []string{"cmd/serve.go is listed but its content was not fetched", "cmd/superuser.go is listed but its content was not fetched"}},
	{file: "Savio09__face-recognition.json", kind: apimodel.KindOther, status: crawler.VerdictInconclusive,
		mentions: []string{"no decisive"}},
}

func TestCorpusThroughThePipeline(t *testing.T) {
	var resolved, correct, incorrect, inconclusive int
	for _, c := range pipelineCases {
		s := loadCorpus(t, c.file)
		t.Run(s.Repo, func(t *testing.T) {
			m := &snapshotServer{s: s}
			a, err := runPipeline(t, m)
			if err != nil {
				t.Fatal(err)
			}
			got := a.classification

			// Acquisition fetched exactly the captured canonical selection, in
			// order, with one requested ref throughout.
			if !reflect.DeepEqual(m.requested, s.Selected) {
				t.Errorf("requested contents differ from the canonical selection:\n got  %v\n want %v", m.requested, s.Selected)
			}
			if len(m.refs) != 1 || !m.refs[s.RequestedRef] {
				t.Errorf("refs used = %v, want only %q", m.refs, s.RequestedRef)
			}
			// A branch-only capture is not an immutable revision.
			if a.insp.Revision.Ref != s.RequestedRef || a.insp.Revision.Resolved {
				t.Errorf("revision = %+v, want the branch %q, unresolved", a.insp.Revision, s.RequestedRef)
			}

			if got.Status == crawler.VerdictInconclusive {
				inconclusive++
			} else {
				resolved++
			}
			match := got.Kind == c.kind && got.Status == c.status
			switch {
			case match && got.Status == crawler.VerdictResolved:
				correct++
			case !match && got.Status == crawler.VerdictResolved:
				incorrect++
			}
			t.Logf("%s: %s/%s (want %s/%s)", s.Repo, got.Kind, got.Status, c.kind, c.status)
			if !match {
				t.Errorf("got %s/%s, want %s/%s:\n%s", got.Kind, got.Status, c.kind, c.status, strings.Join(got.Evidence(), "\n"))
			}
			for _, want := range c.mentions {
				if !containsAny(got.Limitations, want) {
					t.Errorf("no limitation names %q", want)
				}
			}

			// The pack renders the same validated facts, each with its witness.
			md := renderPack(a.repo, a.tree, a.requested, a.fetched, got)
			if !strings.Contains(md, "**Kind:** "+string(got.Kind)) || !strings.Contains(md, "**Status:** "+string(got.Status)) {
				t.Errorf("the pack does not show the verdict")
			}
			for _, o := range got.Observations {
				if !strings.Contains(md, "witness: "+o.Witness.String()) {
					t.Errorf("the pack omits the witness for %q", o.Detail)
				}
			}
			for _, l := range got.Limitations {
				if !strings.Contains(md, l) {
					t.Errorf("the pack omits a limitation: %s", l)
				}
			}
		})
	}
	t.Logf("pipeline corpus: resolved=%d correct=%d incorrect=%d inconclusive=%d (of %d)",
		resolved, correct, incorrect, inconclusive, len(pipelineCases))
}

// Degradations driven through the real client and the mock: each changes only
// what acquisition can see, and the verdict must change accordingly.
func TestCorpusPipelineDegradations(t *testing.T) {
	type mutation struct {
		name    string
		file    string
		setup   func(*snapshotServer)
		kind    apimodel.RepoKind
		status  crawler.VerdictStatus
		mention string
	}
	withoutPath := func(p string) func(*snapshotServer) {
		return func(m *snapshotServer) {
			var kept []corpus.Entry
			for _, e := range m.s.Tree.Entries {
				if e.Path != p {
					kept = append(kept, e)
				}
			}
			m.s.Tree.Entries = kept
		}
	}
	cases := []mutation{
		{"no README: a positive rests on code and manifests", "starship__starship.json", withoutPath("README.md"),
			apimodel.KindCLI, crawler.VerdictResolved, ""},
		{"failed decisive read (Cargo.toml 404)", "starship__starship.json", func(m *snapshotServer) { m.drop = map[string]bool{"Cargo.toml": true} },
			apimodel.KindOther, crawler.VerdictInconclusive, "Cargo.toml could not be read (not found)"},
		{"failed root manifest (package.json 404)", "Savio09__plantly.json", func(m *snapshotServer) { m.drop = map[string]bool{"package.json": true} },
			apimodel.KindOther, crawler.VerdictInconclusive, "package.json could not be read (not found)"},
		{"manifest clipped by the client's per-file cap", "starship__starship.json", func(m *snapshotServer) { m.pad = map[string]string{"Cargo.toml": "[dependencies]"} },
			apimodel.KindOther, crawler.VerdictInconclusive, "Cargo.toml was read only partially"},
		{"truncated tree keeps positive facts", "cli__cli.json", func(m *snapshotServer) { m.s.Tree.Truncated = true },
			apimodel.KindCLI, crawler.VerdictResolved, "the file listing was truncated"},
		{"truncated tree cannot prove a library has no command", "pocketbase__pocketbase.json", func(m *snapshotServer) { m.s.Tree.Truncated = true },
			apimodel.KindOther, crawler.VerdictInconclusive, "the file listing was truncated"},
		{"depth cap drops a deep path and is recorded", "starship__starship.json", func(m *snapshotServer) {
			m.s.Tree.Entries = append(m.s.Tree.Entries, corpus.Entry{Path: strings.Repeat("deep/", 13) + "x.rs", Type: "blob"})
		}, apimodel.KindCLI, crawler.VerdictResolved, "the file listing was depth-limited"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &snapshotServer{s: loadCorpus(t, c.file)}
			c.setup(m)
			a, err := runPipeline(t, m)
			if err != nil {
				t.Fatal(err)
			}
			got := a.classification
			if got.Kind != c.kind || got.Status != c.status {
				t.Errorf("got %s/%s, want %s/%s:\n%s", got.Kind, got.Status, c.kind, c.status, strings.Join(got.Evidence(), "\n"))
			}
			if c.mention != "" && !containsAny(got.Limitations, c.mention) {
				t.Errorf("no limitation names %q: %v", c.mention, got.Limitations)
			}
		})
	}
}

// Cancelling during the last selected file of a real snapshot stops the
// analysis with the context error rather than a verdict built on a skip.
func TestCorpusPipelineFinalFileCancellation(t *testing.T) {
	s := loadCorpus(t, "starship__starship.json")
	last := s.Selected[len(s.Selected)-1]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &snapshotServer{s: s, onContents: func(p string, r *http.Request) {
		if p == last {
			cancel()
			<-r.Context().Done()
		}
	}}
	srv := m.start(t)
	owner, name, _ := strings.Cut(s.Repo, "/")
	c := githubclient.New("t", githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0))
	if _, err := analyze(ctx, c, owner, name); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func containsAny(list []string, sub string) bool {
	for _, l := range list {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}
