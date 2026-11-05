package analysis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Savio09/gittok-ios-backend/internal/githubclient"
)

func TestAcquireUsesCanonicalIdentityAfterRedirectedLookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/alias/old":
			_, _ = w.Write([]byte(`{"id":123,"owner":{"login":"canonical"},"name":"project","full_name":"canonical/project","private":false,"default_branch":"main"}`))
		case "/repos/canonical/project":
			_, _ = w.Write([]byte(`{"id":123,"owner":{"login":"canonical"},"name":"project","full_name":"canonical/project","private":false,"default_branch":"main"}`))
		case "/repos/canonical/project/commits/main":
			_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
		case "/repos/canonical/project/git/trees/0123456789abcdef0123456789abcdef01234567":
			_ = json.NewEncoder(w).Encode(map[string]any{"tree": []any{}})
		default:
			t.Errorf("read reused alias or unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	result, err := NewPublicService(githubclient.WithBaseURL(srv.URL), githubclient.WithMaxRetries(0)).Acquire(context.Background(), "alias", "old")
	if err != nil || !result.Inspection.Revision.Resolved || !strings.HasPrefix(result.Repo.FullName, "canonical/") {
		t.Fatalf("bad canonical result: %+v %v", result, err)
	}
}

func TestPublicPreviewWithholdsTreeOnlyClassificationOnSlugReuse(t *testing.T) {
	reads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r":
			reads++
			id := 1
			if reads > 1 {
				id = 2
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "owner": map[string]string{"login": "o"}, "name": "r", "full_name": "o/r", "private": false, "default_branch": "main"})
		case "/repos/o/r/commits/main":
			_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
		case "/repos/o/r/git/trees/0123456789abcdef0123456789abcdef01234567":
			_, _ = w.Write([]byte(`{"tree":[{"path":"android","type":"tree"},{"path":"ios","type":"tree"}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a, err := NewPublicService(githubclient.WithBaseURL(srv.URL)).Acquire(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	if reads != 2 || len(a.Inspection.Paths) != 0 || a.Inspection.Revision.Resolved {
		t.Fatalf("replacement tree escaped identity check: reads=%d inspection=%+v", reads, a.Inspection)
	}
}

func TestPublicPreviewWithholdsFilesIfCanonicalSlugIsReused(t *testing.T) {
	metadataReads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r":
			metadataReads++
			id := 1
			if metadataReads > 1 {
				id = 2
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"id": id, "owner": map[string]string{"login": "o"}, "name": "r", "full_name": "o/r", "private": false, "default_branch": "main"})
		case "/repos/o/r/commits/main":
			_, _ = w.Write([]byte(`{"sha":"0123456789abcdef0123456789abcdef01234567"}`))
		case "/repos/o/r/git/trees/0123456789abcdef0123456789abcdef01234567":
			_, _ = w.Write([]byte(`{"tree":[{"path":"README.md","type":"blob","mode":"100644"}]}`))
		case "/repos/o/r/contents/README.md":
			_, _ = w.Write([]byte(`{"type":"file","encoding":"base64","content":"cmV1c2VkLXJlcG9zaXRvcnkgc2VjcmV0"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	p, err := NewPublicService(githubclient.WithBaseURL(srv.URL)).Preview(context.Background(), "o", "r")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(p)
	if metadataReads != 2 || strings.Contains(string(b), "reused-repository secret") || !strings.Contains(string(b), "file evidence was withheld") {
		t.Fatalf("mixed repository identity: %s", b)
	}
}
