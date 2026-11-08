package generation

import (
	"bytes"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLedgerOnlyOneDuplicateReservationAuthorizesExecution(t *testing.T) {
	_, _, l := ledgerFixture(t, 100)
	var fresh atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := l.Reserve("job", "writer", 10)
			if err != nil {
				t.Error(err)
			} else if !r.AlreadyReserved {
				fresh.Add(1)
			}
		}()
	}
	wg.Wait()
	if fresh.Load() != 1 {
		t.Fatalf("provider executions authorized = %d, want 1", fresh.Load())
	}
}

func TestLedgerBudgetArithmeticDoesNotOverflow(t *testing.T) {
	_, _, l := ledgerFixture(t, math.MaxInt64)
	if _, err := l.Reserve("job", "one", math.MaxInt64-1); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("job", "two", 2); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("overflowed budget: %v", err)
	}
	if _, err := l.Reserve("job", "two", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("job", "three", 1); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("exhausted budget: %v", err)
	}
}

func TestLedgerOverchargeAfterSettlementStillBlocks(t *testing.T) {
	_, _, l := ledgerFixture(t, 100)
	r, _ := l.Reserve("job", "one", 10)
	if err := l.Settle(r, 5); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(r, 11); !errors.Is(err, ErrLedgerBlocked) {
		t.Fatalf("reconciled overcharge did not block: %v", err)
	}
}

func TestLedgerRejectsInterruptedWriteWithoutDiscardingIt(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	_ = l.Close()
	pending := path + ".pending"
	if err := os.WriteFile(pending, []byte("interrupted accounting"), 0600); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenLedger(path, limits, time.Now); err == nil {
		opened.Close()
		t.Fatal("interrupted write was ignored")
	}
	if err := InitializeLedger(path, limits, limits.WindowID); err == nil {
		t.Fatal("interrupted write was reset")
	}
	if contents, err := os.ReadFile(pending); err != nil || string(contents) != "interrupted accounting" {
		t.Fatalf("reconciliation evidence lost: %q %v", contents, err)
	}
}

func TestLedgerCannotInitializeLostAccountingWithExistingMarker(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	if _, err := l.Reserve("job", "writer", 80); err != nil {
		t.Fatal(err)
	}
	_ = l.Close()
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := InitializeLedger(path, limits, limits.WindowID); err == nil {
		t.Fatal("lost accounting reset despite prior marker")
	}
}

func TestLedgerMissingLockMarkerCannotBeRecreatedByStartup(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	_ = l.Close()
	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenLedger(path, limits, time.Now); err == nil {
		opened.Close()
		t.Fatal("startup recreated lost lock marker")
	}
}

func TestLedgerFsyncFailureNeverAuthorizesProviderAndRetainsReservation(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	if err := l.dir.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("job", "writer", 70); err == nil {
		t.Fatal("provider authorized despite fsync failure")
	}
	if _, err := l.Reserve("job", "writer", 70); err == nil {
		t.Fatal("I/O failure did not fail closed")
	}
	_ = l.Close()
	reopened, err := OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	r, err := reopened.Reserve("job", "writer", 70)
	if err != nil || !r.AlreadyReserved {
		t.Fatalf("uncertain reservation lost: %+v %v", r, err)
	}
	if _, err := reopened.Reserve("other", "writer", 31); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("uncertain reservation refunded: %v", err)
	}
}

func TestLedgerRejectsLedgerAndLockHardlinksOrSymlinks(t *testing.T) {
	for _, suffix := range []string{"", ".lock"} {
		for _, kind := range []string{"hardlink", "symlink"} {
			t.Run(suffix+kind, func(t *testing.T) {
				path, limits, l := ledgerFixture(t, 100)
				_ = l.Close()
				if kind == "hardlink" {
					if err := os.Link(path+suffix, path+suffix+".alias"); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Rename(path+suffix, path+suffix+".target"); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(path+suffix+".target", path+suffix); err != nil {
						t.Fatal(err)
					}
				}
				if opened, err := OpenLedger(path, limits, time.Now); err == nil {
					opened.Close()
					t.Fatal("unsafe accounting file opened")
				}
			})
		}
	}
}

func TestLedgerRejectsSymlinkedParentDirectory(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	_ = l.Close()
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Dir(path), link); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenLedger(filepath.Join(link, filepath.Base(path)), limits, time.Now); err == nil {
		opened.Close()
		t.Fatal("symlinked accounting directory opened")
	}
}

func TestLedgerContainsOnlyBoundedHashedAccounting(t *testing.T) {
	path, _, l := ledgerFixture(t, 100)
	if _, err := l.Reserve("sensitive-job-identity", "sensitive-call-identity", 1); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contents, []byte("sensitive-")) {
		t.Fatal("raw caller identity persisted")
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("unsafe permissions: %v %v", info, err)
	}
	files, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(files) != 2 {
		t.Fatalf("unbounded accounting files: %v %v", files, err)
	}
}

func TestLedgerSecondProcessCannotOwnSamePath(t *testing.T) {
	path, _, _ := ledgerFixture(t, 100)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestLedgerProcessLockHelper$", "--", path)
	cmd.Env = []string{"GITTOK_LEDGER_LOCK_HELPER=1"}
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("second-process lock failed: %v %s", err, output)
	}
}

func TestLedgerProcessLockHelper(t *testing.T) {
	if os.Getenv("GITTOK_LEDGER_LOCK_HELPER") != "1" {
		return
	}
	limits := LedgerLimits{MaxWindowMicros: 100, WindowID: "operator-window-1", MaxReservations: 32}
	ledger, err := OpenLedger(os.Args[len(os.Args)-1], limits, time.Now)
	if ledger != nil {
		ledger.Close()
	}
	if !errors.Is(err, ErrLedgerLocked) {
		t.Fatalf("second process obtained accounting ownership: %v", err)
	}
}
