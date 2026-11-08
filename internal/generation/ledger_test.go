package generation

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func ledgerFixture(t *testing.T, amount int64) (string, LedgerLimits, *Ledger) {
	t.Helper()
	path := filepath.Join(ledgerDirectory(t), "spend.json")
	limits := LedgerLimits{MaxWindowMicros: amount, WindowID: "operator-window-1", MaxReservations: 32}
	if err := InitializeLedger(path, limits, limits.WindowID); err != nil {
		t.Fatal(err)
	}
	l, err := OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return path, limits, l
}

func ledgerDirectory(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLedgerReservationSurvivesRestartWithoutRefundOrReplay(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	first, err := l.Reserve("job", "writer", 70)
	if err != nil || first.AlreadyReserved {
		t.Fatalf("first: %+v, %v", first, err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	l, err = OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	duplicate, err := l.Reserve("job", "writer", 70)
	if err != nil || !duplicate.AlreadyReserved || duplicate.AlreadySettled {
		t.Fatalf("duplicate: %+v, %v", duplicate, err)
	}
	if _, err := l.Reserve("job", "reviewer", 31); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("lost outstanding spend: %v", err)
	}
	if _, err := l.Reserve("job", "writer", 69); !errors.Is(err, ErrReservationConflict) {
		t.Fatalf("changed duplicate: %v", err)
	}
}

func TestLedgerVerifiedSettlementReleasesOnlyDifferenceAndNeverReplays(t *testing.T) {
	_, _, l := ledgerFixture(t, 100)
	r, err := l.Reserve("job", "writer", 70)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(r, 40); err != nil {
		t.Fatal(err)
	}
	if err := l.Settle(r, 40); err != nil {
		t.Fatalf("identical settlement: %v", err)
	}
	if err := l.Settle(r, 39); !errors.Is(err, ErrReservationConflict) {
		t.Fatalf("settlement rewrite: %v", err)
	}
	duplicate, err := l.Reserve("job", "writer", 70)
	if err != nil || !duplicate.AlreadyReserved || !duplicate.AlreadySettled {
		t.Fatalf("settled replay: %+v, %v", duplicate, err)
	}
	if _, err := l.Reserve("job", "reviewer", 60); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("other", "writer", 1); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatalf("overspend: %v", err)
	}
}

func TestLedgerOverchargeBlocksDurably(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	r, _ := l.Reserve("job", "writer", 50)
	if err := l.Settle(r, 51); !errors.Is(err, ErrLedgerBlocked) {
		t.Fatalf("overcharge: %v", err)
	}
	if _, err := l.Reserve("job", "reviewer", 1); !errors.Is(err, ErrLedgerBlocked) {
		t.Fatalf("blocked reserve: %v", err)
	}
	_ = l.Close()
	if reopened, err := OpenLedger(path, limits, time.Now); !errors.Is(err, ErrLedgerBlocked) {
		if reopened != nil {
			reopened.Close()
		}
		t.Fatalf("reopened overcharged ledger: %v", err)
	}
}

func TestLedgerFailsClosedOnMissingEmptyCorruptOrTruncatedState(t *testing.T) {
	for _, state := range []string{"missing", "", "{}", "{", "null", "[]"} {
		t.Run(state, func(t *testing.T) {
			path := filepath.Join(ledgerDirectory(t), "spend.json")
			if state != "missing" {
				if err := os.WriteFile(path, []byte(state), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if l, err := OpenLedger(path, LedgerLimits{MaxWindowMicros: 100, WindowID: "window"}, time.Now); err == nil {
				l.Close()
				t.Fatal("unsafe ledger opened")
			}
		})
	}
}

func TestLedgerRejectsResetWindowMismatchAndUnsafeFiles(t *testing.T) {
	path, limits, l := ledgerFixture(t, 100)
	if err := InitializeLedger(path, limits, limits.WindowID); err == nil {
		t.Fatal("initialization reset existing accounting")
	}
	_ = l.Close()
	wrong := limits
	wrong.WindowID = "new-window"
	if opened, err := OpenLedger(path, wrong, time.Now); err == nil {
		opened.Close()
		t.Fatal("automatic window reset")
	}
	wrong = limits
	wrong.MaxWindowMicros++
	if opened, err := OpenLedger(path, wrong, time.Now); err == nil {
		opened.Close()
		t.Fatal("limit silently changed")
	}
	link := filepath.Join(filepath.Dir(path), "symlink.json")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenLedger(link, limits, time.Now); err == nil {
		opened.Close()
		t.Fatal("symlink opened")
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if opened, err := OpenLedger(path, limits, time.Now); err == nil {
		opened.Close()
		t.Fatal("public accounting file opened")
	}
}

func TestLedgerConcurrentReservationsCannotOverspendAndSecondOwnerDenied(t *testing.T) {
	path, limits, l := ledgerFixture(t, 10)
	if other, err := OpenLedger(path, limits, time.Now); !errors.Is(err, ErrLedgerLocked) {
		if other != nil {
			other.Close()
		}
		t.Fatalf("second owner: %v", err)
	}
	var success atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := l.Reserve("job", time.Unix(int64(i), 0).Format(time.RFC3339), 1); err == nil {
				success.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if success.Load() != 10 {
		t.Fatalf("successful calls = %d, want 10", success.Load())
	}
}

func TestLedgerRejectsInvalidBoundsAndCapacity(t *testing.T) {
	for _, limits := range []LedgerLimits{{}, {MaxWindowMicros: -1, WindowID: "w"}, {MaxWindowMicros: 1}, {MaxWindowMicros: 1, WindowID: "w", MaxReservations: -1}} {
		if err := InitializeLedger(filepath.Join(ledgerDirectory(t), "spend.json"), limits, limits.WindowID); err == nil {
			t.Fatalf("invalid limits: %+v", limits)
		}
	}
	_, _, l := ledgerFixture(t, 100)
	for _, amount := range []int64{-1, 0, 101} {
		if _, err := l.Reserve("job", "call", amount); err == nil {
			t.Fatalf("invalid amount %d", amount)
		}
	}
	r, _ := l.Reserve("job", "call", 1)
	if err := l.Settle(r, -1); err == nil {
		t.Fatal("negative settlement accepted")
	}
	if err := l.Settle(Reservation{}, 0); err == nil {
		t.Fatal("forged settlement accepted")
	}
	path := filepath.Join(ledgerDirectory(t), "capacity.json")
	limits := LedgerLimits{MaxWindowMicros: 100, WindowID: "w", MaxReservations: 1}
	if err := InitializeLedger(path, limits, "w"); err != nil {
		t.Fatal(err)
	}
	capLedger, err := OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer capLedger.Close()
	r, _ = capLedger.Reserve("job", "one", 1)
	if err := capLedger.Settle(r, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := capLedger.Reserve("job", "two", 1); err == nil {
		t.Fatal("record capacity exceeded")
	}
}

func TestLedgerDetectsStateLossWhileOpen(t *testing.T) {
	path, _, l := ledgerFixture(t, 100)
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Reserve("job", "writer", 10); err == nil {
		t.Fatal("paid call permitted after accounting loss")
	}
}
