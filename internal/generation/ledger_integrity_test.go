package generation

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLedgerRejectsForeignSnapshotWithoutDiscardingSpend(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	if _, err := l.Reserve("job", "writer", 80); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	otherPath, _, other := ledgerFixture(t, 100)
	if err := other.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(otherPath)
	if err != nil {
		t.Fatal(err)
	}
	// Simulate a restore mixing one ledger's snapshot with another's marker.
	replacement := path + ".replacement"
	if err := os.WriteFile(replacement, contents, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenLedger(path, limits, time.Now); !errors.Is(err, ErrLedgerUnavailable) {
		if opened != nil {
			opened.Close()
		}
		t.Fatalf("foreign snapshot erased prior accounting: %v", err)
	}
}

func TestLedgerRejectsNonPrivateDirectory(t *testing.T) {
	for _, mode := range []os.FileMode{0755, 0711, 0750, 0777} {
		t.Run(mode.String(), func(t *testing.T) {
			path, limits, l := ledgerFixture(t, 100)
			if err := os.Chmod(filepath.Dir(path), mode); err != nil {
				t.Fatal(err)
			}
			if _, err := l.Reserve("job", "writer", 10); !errors.Is(err, ErrLedgerUnavailable) {
				t.Errorf("paid work permitted after directory became %v: %v", mode, err)
			}
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			if opened, err := OpenLedger(path, limits, time.Now); !errors.Is(err, ErrLedgerUnavailable) {
				if opened != nil {
					opened.Close()
				}
				t.Errorf("opened ledger in directory mode %v: %v", mode, err)
			}
			if err := InitializeLedger(filepath.Join(filepath.Dir(path), "new.json"), limits, limits.WindowID); !errors.Is(err, ErrLedgerUnavailable) {
				t.Errorf("initialized ledger in directory mode %v: %v", mode, err)
			}
		})
	}
}

func TestLedgerRejectsEmptyCorruptOrOversizedMarker(t *testing.T) {
	for _, contents := range []string{
		"", "{", "null", "[]", "{}", strings.Repeat("x", 1024),
		`{"version":2,"ledger_id":"00000000000000000000000000000000"}`,
		`{"version":1,"ledger_id":"gggggggggggggggggggggggggggggggg"}`,
		`{"version":1,"ledger_id":"00000000000000000000000000000000","unknown":true}`,
	} {
		t.Run(contents[:min(len(contents), 8)], func(t *testing.T) {
			path, limits, l := ledgerFixture(t, 100)
			if err := l.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+".lock", []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			if opened, err := OpenLedger(path, limits, time.Now); !errors.Is(err, ErrLedgerUnavailable) {
				if opened != nil {
					opened.Close()
				}
				t.Fatalf("invalid marker accepted: %v", err)
			}
			if err := InitializeLedger(path, limits, limits.WindowID); err == nil {
				t.Fatal("invalid marker was silently reinitialized")
			}
		})
	}
}

func TestLedgerDetectsMarkerCorruptionWhileOpen(t *testing.T) {
	path, _, l := ledgerFixture(t, 100)
	if err := os.WriteFile(path+".lock", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("job", "writer", 80); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("paid work permitted after marker loss: %v", err)
	}
}

func TestLedgerDetectsForeignMarkerWhileOpen(t *testing.T) {
	path, _, l := ledgerFixture(t, 100)
	otherPath, _, _ := ledgerFixture(t, 100)
	contents, err := os.ReadFile(otherPath + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	// Keep the held inode but replace its identity with a different valid marker.
	if err := os.WriteFile(path+".lock", contents, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("job", "writer", 80); !errors.Is(err, ErrLedgerUnavailable) {
		t.Fatalf("paid work permitted with mismatched marker: %v", err)
	}
}

func TestLedgerMarkerInodeAndIdentityRemainStableThroughAccounting(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	before, err := os.Stat(path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	marker, err := os.ReadFile(path + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	r, err := l.Reserve("job", "writer", 80)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(r, 40); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	after, err := os.Stat(path + ".lock")
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("accounting replaced the flock inode: %v", err)
	}
	afterMarker, err := os.ReadFile(path + ".lock")
	if err != nil || string(marker) != string(afterMarker) {
		t.Fatalf("accounting rewrote the control identity: %v", err)
	}
}
