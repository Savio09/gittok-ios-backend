package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
)

type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time { return c.t }

func testLimits() CacheLimits {
	return CacheLimits{TTL: 30 * time.Minute, MaxEntries: 128, MaxBytes: 32 << 20, MaxEntryBytes: 512 << 10}
}

func openTestCache(t *testing.T, dir string, l CacheLimits, clock *fakeClock) *PreviewCache {
	t.Helper()
	c, err := OpenPreviewCache(dir, l, clock.now)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func samplePreview(name string) apimodel.RepoPreview {
	return apimodel.RepoPreview{ID: "o/" + name, Owner: "o", Name: name, Kind: apimodel.KindCLI, Description: "public description"}
}

func key(id int64, name string) CacheKey { return CacheKey{RepoID: id, Owner: "o", Name: name} }

func TestCacheServesWithinTTLAndExpires(t *testing.T) {
	clock := &fakeClock{time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)}
	dir := t.TempDir()
	c := openTestCache(t, dir, testLimits(), clock)
	if _, ok := c.Get(key(1, "r")); ok {
		t.Fatal("cold cache hit")
	}
	c.Put(key(1, "r"), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview("r"))
	if p, ok := c.Get(key(1, "r")); !ok || p.Description != "public description" {
		t.Fatalf("warm miss: %+v %v", p, ok)
	}
	clock.t = clock.t.Add(31 * time.Minute)
	if _, ok := c.Get(key(1, "r")); ok {
		t.Fatal("served an expired entry")
	}
	if files, _ := filepath.Glob(filepath.Join(dir, "*.json")); len(files) != 0 {
		t.Fatalf("expired entry left on disk: %v", files)
	}
}

func TestCacheKeyIncludesStableIdentityAndBuilderVersion(t *testing.T) {
	clock := &fakeClock{time.Now()}
	c := openTestCache(t, t.TempDir(), testLimits(), clock)
	c.Put(key(1, "r"), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview("r"))
	for name, k := range map[string]CacheKey{
		"reused name, new repository": key(2, "r"),
		"renamed repository":          key(1, "renamed"),
		"other owner":                 {RepoID: 1, Owner: "x", Name: "r"},
		"older builder":               {RepoID: 1, Owner: "o", Name: "r", version: builderVersion() + 1},
	} {
		if _, ok := c.Get(k); ok {
			t.Errorf("%s: served another identity's entry", name)
		}
	}
}

func TestCachePersistsAcrossRestart(t *testing.T) {
	clock := &fakeClock{time.Now()}
	dir := t.TempDir()
	openTestCache(t, dir, testLimits(), clock).Put(key(1, "r"), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview("r"))
	if _, ok := openTestCache(t, dir, testLimits(), clock).Get(key(1, "r")); !ok {
		t.Fatal("entry lost across restart")
	}
}

func TestCacheEvictsWithinEntryAndByteBounds(t *testing.T) {
	clock := &fakeClock{time.Now()}
	l := testLimits()
	l.MaxEntries = 2
	c := openTestCache(t, t.TempDir(), l, clock)
	for i, name := range []string{"a", "b", "c"} {
		clock.t = clock.t.Add(time.Second)
		c.Put(key(int64(i+1), name), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview(name))
	}
	if _, ok := c.Get(key(1, "a")); ok {
		t.Error("the oldest entry survived past the entry bound")
	}
	if _, ok := c.Get(key(3, "c")); !ok {
		t.Error("the newest entry was evicted")
	}
	// A normal entry is about 300 bytes: a 500-byte bound holds exactly one.
	l = testLimits()
	l.MaxBytes, l.MaxEntryBytes = 500, 500
	c = openTestCache(t, t.TempDir(), l, clock)
	if !c.Put(key(1, "a"), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview("a")) {
		t.Fatal("fixture entry does not fit the byte bound")
	}
	clock.t = clock.t.Add(time.Second)
	c.Put(key(2, "b"), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview("b"))
	_, oldKept := c.Get(key(1, "a"))
	_, newKept := c.Get(key(2, "b"))
	if c.Bytes() > 500 || oldKept || !newKept {
		t.Errorf("byte bound: %d bytes, old kept %v, new kept %v", c.Bytes(), oldKept, newKept)
	}
}

func TestCacheRejectsOversizedCorruptAndForeignEntries(t *testing.T) {
	clock := &fakeClock{time.Now()}
	dir := t.TempDir()
	l := testLimits()
	l.MaxEntryBytes = 600 // a normal entry (~300 bytes) fits; the large one does not
	c := openTestCache(t, dir, l, clock)
	big := samplePreview("r")
	big.Description = strings.Repeat("x", 1000)
	if c.Put(key(1, "r"), "0123456789abcdef0123456789abcdef01234567", clock.t, big) {
		t.Error("stored an entry over the size limit")
	}
	if _, ok := c.Get(key(1, "r")); ok {
		t.Error("served an oversized entry")
	}
	foreign := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(foreign, []byte("keep me"), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Put(key(2, "s"), "0123456789abcdef0123456789abcdef01234567", clock.t, samplePreview("s"))
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("entries on disk: %v", files)
	}
	if err := os.WriteFile(files[0], []byte("{corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := openTestCache(t, dir, l, clock).Get(key(2, "s")); ok {
		t.Error("served a corrupt entry")
	}
	if _, err := os.Stat(foreign); err != nil {
		t.Error("the cache deleted a file it does not own")
	}
	if tmp, _ := filepath.Glob(filepath.Join(dir, ".tmp-*")); len(tmp) != 0 {
		t.Errorf("temporary files left behind: %v", tmp)
	}
}

func TestCacheValidatesLimitsAndItsDirectory(t *testing.T) {
	clock := &fakeClock{time.Now()}
	for name, l := range map[string]CacheLimits{
		"zero TTL":           {TTL: 0, MaxEntries: 1, MaxBytes: 1, MaxEntryBytes: 1},
		"no entries":         {TTL: time.Minute, MaxEntries: 0, MaxBytes: 1, MaxEntryBytes: 1},
		"no bytes":           {TTL: time.Minute, MaxEntries: 1, MaxBytes: 0, MaxEntryBytes: 1},
		"entry over total":   {TTL: time.Minute, MaxEntries: 1, MaxBytes: 1, MaxEntryBytes: 2},
		"negative entry cap": {TTL: time.Minute, MaxEntries: 1, MaxBytes: 1, MaxEntryBytes: -1},
	} {
		if _, err := OpenPreviewCache(t.TempDir(), l, clock.now); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable")
	}
	if _, err := OpenPreviewCache(link, testLimits(), clock.now); err == nil {
		t.Error("accepted a symlinked cache directory")
	}
	fresh := filepath.Join(t.TempDir(), "cache")
	if _, err := OpenPreviewCache(fresh, testLimits(), clock.now); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(fresh); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("cache directory permissions: %v %v", info.Mode(), err)
	}
}
