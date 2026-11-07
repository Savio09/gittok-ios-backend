package generation

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestExplicitBudgetIncreasePreservesChargesUnknownReservationsAndReplay(t *testing.T) {
	path, limits, l := ledgerFixture(t, 1_000_000)
	settled, err := l.Reserve("settled-job", "call", 300_000)
	if err != nil {
		t.Fatal(err)
	}
	if err = l.Settle(settled, 100_000); err != nil {
		t.Fatal(err)
	}
	unknown, err := l.Reserve("unknown-job", "call", 500_000)
	if err != nil {
		t.Fatal(err)
	}
	before := l.state
	if err = l.IncreaseWindowBudget(1_000_000, 2_000_000); err != nil {
		t.Fatal(err)
	}
	if l.state.ID != before.ID || l.state.WindowID != before.WindowID || l.state.Blocked != before.Blocked || !reflect.DeepEqual(l.state.Entries, before.Entries) {
		t.Fatal("operator increase changed accounting history")
	}
	duplicate, err := l.Reserve("unknown-job", "call", 500_000)
	if err != nil || !duplicate.AlreadyReserved || duplicate.AlreadySettled || duplicate.key != unknown.key {
		t.Fatal("increase released or replayed an unknown charge", err)
	}
	if _, err = l.Reserve("new-job", "call", 1_400_001); !errors.Is(err, ErrBudgetExceeded) {
		t.Fatal("existing charges were not deducted under raised ceiling", err)
	}
	if err = l.Close(); err != nil {
		t.Fatal(err)
	}
	if reopened, err := OpenLedger(path, limits, time.Now); err == nil {
		reopened.Close()
		t.Fatal("startup silently accepted stale operator cap")
	}
	limits.MaxWindowMicros = 2_000_000
	reopened, err := OpenLedger(path, limits, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if !reflect.DeepEqual(reopened.state.Entries, before.Entries) {
		t.Fatal("restart lost retained charges")
	}
}

func TestBudgetIncreaseRequiresExactOldCapAndStrictIncrease(t *testing.T) {
	_, _, l := ledgerFixture(t, 1_000_000)
	before := l.state
	for _, values := range [][2]int64{{0, 2_000_000}, {999_999, 2_000_000}, {1_000_000, 1_000_000}, {1_000_000, 500_000}, {1_000_000, 1_000_000_001}} {
		if err := l.IncreaseWindowBudget(values[0], values[1]); err == nil {
			t.Fatal("invalid operator increase accepted", values)
		}
		if !reflect.DeepEqual(l.state, before) {
			t.Fatal("rejected increase changed accounting")
		}
	}
}
