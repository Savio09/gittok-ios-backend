package analysis

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Savio09/gittok-ios-backend/internal/apimodel"
	"github.com/Savio09/gittok-ios-backend/internal/preview"
)

// This file is a bounded on-disk store of public previews. It holds only the
// rendered public preview and its provenance (repository ID, canonical name,
// commit, capture and expiry times) — never credentials, private contents or
// provider payloads. It is not a source of truth about visibility: the service
// reconfirms current public metadata before every lookup.

// CacheLimits bounds the store. All values must be positive, and one entry
// may not exceed the total byte bound.
type CacheLimits struct {
	TTL           time.Duration
	MaxEntries    int
	MaxBytes      int64
	MaxEntryBytes int
}

// DefaultCacheLimits are the operating defaults.
func DefaultCacheLimits() CacheLimits {
	return CacheLimits{TTL: 30 * time.Minute, MaxEntries: 128, MaxBytes: 32 << 20, MaxEntryBytes: 512 << 10}
}

// DefaultCacheDir is a dedicated directory under the system temporary
// directory. On an ephemeral filesystem it does not survive a redeploy.
func DefaultCacheDir() string { return filepath.Join(os.TempDir(), "gittok-preview-cache") }

// Validate rejects a non-positive or inconsistent bound.
func (l CacheLimits) Validate() error {
	switch {
	case l.TTL <= 0:
		return errors.New("cache TTL must be positive")
	case l.MaxEntries <= 0 || l.MaxBytes <= 0 || l.MaxEntryBytes <= 0:
		return errors.New("cache capacities must be positive")
	case int64(l.MaxEntryBytes) > l.MaxBytes:
		return errors.New("a cache entry may not exceed the total cache size")
	}
	return nil
}

// CacheKey is a preview's identity: the stable repository ID together with its
// current canonical owner/name and the builder version. A reused name (new ID),
// a rename (new name) or a newer builder never matches an older entry.
type CacheKey struct {
	RepoID      int64
	Owner, Name string
	version     int // 0 means the current builder version
}

func builderVersion() int { return preview.BuilderVersion }

func (k CacheKey) builder() int {
	if k.version != 0 {
		return k.version
	}
	return builderVersion()
}

func (k CacheKey) file() string {
	sum := sha256.Sum256([]byte(strconv.FormatInt(k.RepoID, 10) + "\x00" + k.Owner + "\x00" + k.Name + "\x00" + strconv.Itoa(k.builder())))
	return hex.EncodeToString(sum[:]) + ".json"
}

type cacheEntry struct {
	Version    int                  `json:"version"`
	RepoID     int64                `json:"repo_id"`
	Owner      string               `json:"owner"`
	Name       string               `json:"name"`
	Commit     string               `json:"commit"`
	CapturedAt time.Time            `json:"captured_at"`
	Expires    time.Time            `json:"expires"`
	Preview    apimodel.RepoPreview `json:"preview"`
}

type indexed struct {
	size    int64
	expires time.Time
}

// PreviewCache is safe for concurrent use within one process.
type PreviewCache struct {
	dir    string
	limits CacheLimits
	now    func() time.Time

	mu    sync.Mutex
	index map[string]indexed // owned entry file name -> size and expiry
	bytes int64
}

// OpenPreviewCache opens (creating if needed) a dedicated cache directory and
// indexes the entries it already holds, discarding corrupt, oversized or
// expired ones. The directory must not be a symlink; it is kept at 0700.
func OpenPreviewCache(dir string, limits CacheLimits, now func() time.Time) (*PreviewCache, error) {
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	if dir == "" {
		return nil, errors.New("cache directory is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, errors.New("cache directory must be a real directory, not a symlink")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	c := &PreviewCache{dir: dir, limits: limits, now: now, index: map[string]indexed{}}
	names, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	for _, path := range names {
		name := filepath.Base(path)
		if !ownedEntryName(name) {
			continue
		}
		e, size, err := c.read(name)
		if err != nil || !now().Before(e.Expires) {
			_ = os.Remove(path)
			continue
		}
		c.index[name] = indexed{size: size, expires: e.Expires}
		c.bytes += size
	}
	// Stale temporary files from an interrupted write are ours to remove.
	if tmps, err := filepath.Glob(filepath.Join(dir, ".tmp-*")); err == nil {
		for _, t := range tmps {
			_ = os.Remove(t)
		}
	}
	c.mu.Lock()
	c.evictLocked()
	c.mu.Unlock()
	return c, nil
}

// ownedEntryName matches only the file names this cache writes.
func ownedEntryName(name string) bool {
	hexPart, ok := strings.CutSuffix(name, ".json")
	if !ok || len(hexPart) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hexPart)
	return err == nil
}

// read decodes one entry with a bounded read.
func (c *PreviewCache) read(name string) (cacheEntry, int64, error) {
	f, err := os.Open(filepath.Join(c.dir, name))
	if err != nil {
		return cacheEntry{}, 0, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, int64(c.limits.MaxEntryBytes)+1))
	if err != nil {
		return cacheEntry{}, 0, err
	}
	if len(b) > c.limits.MaxEntryBytes {
		return cacheEntry{}, 0, errors.New("cache entry exceeds the size limit")
	}
	var e cacheEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return cacheEntry{}, 0, err
	}
	return e, int64(len(b)), nil
}

// Get returns the cached preview for k while it is current. An entry that is
// expired, corrupt, from another builder version or for another identity is
// removed and reported as a miss.
func (c *PreviewCache) Get(k CacheKey) (apimodel.RepoPreview, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	name := k.file()
	if _, ok := c.index[name]; !ok {
		return apimodel.RepoPreview{}, false
	}
	e, _, err := c.read(name)
	if err != nil || e.Version != k.builder() || e.RepoID != k.RepoID || e.Owner != k.Owner || e.Name != k.Name || !c.now().Before(e.Expires) {
		c.removeLocked(name)
		return apimodel.RepoPreview{}, false
	}
	return e.Preview, true
}

// Put stores p for k, captured at capturedAt from commit, and reports whether
// it was stored. An entry over the size limit is not stored. Writes are atomic:
// a reader sees the previous entry or the new one, never a partial file.
func (c *PreviewCache) Put(k CacheKey, commit string, capturedAt time.Time, p apimodel.RepoPreview) bool {
	now := c.now()
	e := cacheEntry{Version: k.builder(), RepoID: k.RepoID, Owner: k.Owner, Name: k.Name, Commit: commit, CapturedAt: capturedAt.UTC(), Expires: now.Add(c.limits.TTL).UTC(), Preview: p}
	b, err := json.Marshal(e)
	if err != nil || len(b) > c.limits.MaxEntryBytes {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	tmp, err := os.CreateTemp(c.dir, ".tmp-*")
	if err != nil {
		return false
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	name := k.file()
	if werr != nil || cerr != nil || os.Chmod(tmp.Name(), 0o600) != nil || os.Rename(tmp.Name(), filepath.Join(c.dir, name)) != nil {
		_ = os.Remove(tmp.Name())
		return false
	}
	if old, ok := c.index[name]; ok {
		c.bytes -= old.size
	}
	c.index[name] = indexed{size: int64(len(b)), expires: e.Expires}
	c.bytes += int64(len(b))
	c.evictLocked()
	_, kept := c.index[name]
	return kept
}

// Bytes reports the indexed size of the stored entries.
func (c *PreviewCache) Bytes() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.bytes
}

func (c *PreviewCache) removeLocked(name string) {
	if e, ok := c.index[name]; ok {
		c.bytes -= e.size
		delete(c.index, name)
	}
	_ = os.Remove(filepath.Join(c.dir, name))
}

// evictLocked drops expired entries, then the entries nearest expiry (the
// oldest writes), until both bounds hold.
func (c *PreviewCache) evictLocked() {
	now := c.now()
	for name, e := range c.index {
		if !now.Before(e.expires) {
			c.removeLocked(name)
		}
	}
	if len(c.index) <= c.limits.MaxEntries && c.bytes <= c.limits.MaxBytes {
		return
	}
	names := make([]string, 0, len(c.index))
	for name := range c.index {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := c.index[names[i]], c.index[names[j]]
		if !a.expires.Equal(b.expires) {
			return a.expires.Before(b.expires)
		}
		return names[i] < names[j]
	})
	for _, name := range names {
		if len(c.index) <= c.limits.MaxEntries && c.bytes <= c.limits.MaxBytes {
			return
		}
		c.removeLocked(name)
	}
}
