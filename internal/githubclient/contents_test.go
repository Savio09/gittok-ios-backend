package githubclient

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// fileJSON builds a GitHub contents response for a file, base64-encoding the
// content the way GitHub does (with line breaks every 60 chars).
func fileJSON(content string) string {
	b64 := base64.StdEncoding.EncodeToString([]byte(content))
	var wrapped strings.Builder
	for i := 0; i < len(b64); i += 60 {
		end := i + 60
		if end > len(b64) {
			end = len(b64)
		}
		wrapped.WriteString(b64[i:end])
		wrapped.WriteString("\n")
	}
	return fmt.Sprintf(`{"type":"file","encoding":"base64","size":%d,"content":%q}`, len(content), wrapped.String())
}

// contentsHandler serves files by path from a map; unknown paths 404.
func contentsHandler(files map[string]string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/repos/o/r/contents/")
		body, ok := files[path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(body))
	}
}

func skippedReason(f FetchedContents, path string) (string, bool) {
	for _, s := range f.Skipped {
		if s.Path == path {
			return s.Reason, true
		}
	}
	return "", false
}

func TestGetFileContentsDecodesBase64AndStripsNewlines(t *testing.T) {
	src := "package main\n\nfunc main() {}\n"
	var gotPath, gotRef string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotRef = r.URL.Query().Get("ref")
		_, _ = w.Write([]byte(fileJSON(src)))
	})

	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"a.go"}, ContentLimits{})
	if err != nil {
		t.Fatalf("GetFileContents error = %v", err)
	}
	if gotPath != "/repos/o/r/contents/a.go" {
		t.Errorf("request path = %q", gotPath)
	}
	if gotRef != "main" {
		t.Errorf("ref = %q, want main", gotRef)
	}
	if len(got.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(got.Files))
	}
	f := got.Files[0]
	if f.Path != "a.go" || f.Content != src || f.Truncated {
		t.Errorf("file = %+v, want a.go with decoded content, not truncated", f)
	}
	if f.Size != len(src) {
		t.Errorf("size = %d, want %d", f.Size, len(src))
	}
}

func TestGetFileContentsTruncatesLargeFile(t *testing.T) {
	src := strings.Repeat("x", 1000)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(fileJSON(src)))
	})

	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"big.txt"}, ContentLimits{MaxFileBytes: 100})
	if err != nil {
		t.Fatalf("GetFileContents error = %v", err)
	}
	if len(got.Files) != 1 {
		t.Fatalf("files = %d, want 1", len(got.Files))
	}
	if len(got.Files[0].Content) != 100 || !got.Files[0].Truncated {
		t.Errorf("content len = %d truncated = %v, want 100 / true", len(got.Files[0].Content), got.Files[0].Truncated)
	}
}

func TestGetFileContentsSkipsOversizeBlob(t *testing.T) {
	// GitHub returns encoding "none" with empty content for files over ~1MB.
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"type":"file","encoding":"none","size":2000000,"content":""}`))
	})

	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"huge.bin"}, ContentLimits{})
	if err != nil {
		t.Fatalf("GetFileContents error = %v", err)
	}
	if len(got.Files) != 0 {
		t.Errorf("files = %d, want 0 (oversize skipped)", len(got.Files))
	}
	if reason, ok := skippedReason(got, "huge.bin"); !ok || !strings.Contains(reason, "large") {
		t.Errorf("skipped reason = %q (ok=%v), want it to mention large", reason, ok)
	}
}

func TestGetFileContentsSkipsMissingFile(t *testing.T) {
	c := newTestClient(t, contentsHandler(map[string]string{"ok.go": fileJSON("ok")}))

	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"ok.go", "missing.go"}, ContentLimits{})
	if err != nil {
		t.Fatalf("GetFileContents error = %v", err)
	}
	if len(got.Files) != 1 || got.Files[0].Path != "ok.go" {
		t.Errorf("files = %+v, want only ok.go", got.Files)
	}
	if reason, ok := skippedReason(got, "missing.go"); !ok || !strings.Contains(reason, "not found") {
		t.Errorf("missing.go skipped reason = %q (ok=%v)", reason, ok)
	}
}

func TestGetFileContentsRespectsMaxFiles(t *testing.T) {
	c := newTestClient(t, contentsHandler(map[string]string{
		"a.go": fileJSON("a"), "b.go": fileJSON("b"), "c.go": fileJSON("c"),
	}))

	got, err := c.GetFileContents(context.Background(), "o", "r", "main", []string{"a.go", "b.go", "c.go"}, ContentLimits{MaxFiles: 2})
	if err != nil {
		t.Fatalf("GetFileContents error = %v", err)
	}
	if len(got.Files) != 2 {
		t.Errorf("files = %d, want 2 (MaxFiles)", len(got.Files))
	}
	if _, ok := skippedReason(got, "c.go"); !ok {
		t.Errorf("c.go should be skipped once the file limit is reached")
	}
}

func TestGetFileContentsRespectsTotalBudget(t *testing.T) {
	c := newTestClient(t, contentsHandler(map[string]string{
		"a.txt": fileJSON(strings.Repeat("a", 100)),
		"b.txt": fileJSON(strings.Repeat("b", 100)),
	}))

	// Budget 120: a.txt (100) fits; b.txt is clipped to the remaining 20.
	got, err := c.GetFileContents(context.Background(), "o", "r", "main",
		[]string{"a.txt", "b.txt"}, ContentLimits{MaxFileBytes: 1000, MaxTotalBytes: 120})
	if err != nil {
		t.Fatalf("GetFileContents error = %v", err)
	}
	if len(got.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(got.Files))
	}
	if len(got.Files[0].Content) != 100 {
		t.Errorf("a.txt len = %d, want 100", len(got.Files[0].Content))
	}
	if len(got.Files[1].Content) != 20 || !got.Files[1].Truncated {
		t.Errorf("b.txt len = %d truncated = %v, want 20 / true (budget)", len(got.Files[1].Content), got.Files[1].Truncated)
	}
}
