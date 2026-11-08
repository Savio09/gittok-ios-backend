// Package generation provides durable, conservative accounting before paid work.
package generation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

var (
	ErrBudgetExceeded      = errors.New("generation spending budget exhausted")
	ErrLedgerUnavailable   = errors.New("generation spend ledger unavailable")
	ErrLedgerBlocked       = errors.New("generation spend ledger requires reconciliation")
	ErrLedgerLocked        = errors.New("generation spend ledger already owned")
	ErrReservationConflict = errors.New("generation reservation conflicts with prior accounting")
	ErrInvalidReservation  = errors.New("invalid generation reservation")
)

// LedgerLimits is operator configuration, independent of ephemeral job storage.
// WindowID never changes automatically. Zero MaxReservations defaults to 1024;
// settled entries remain until operator reconciliation to prevent paid replay.
type LedgerLimits struct {
	MaxWindowMicros int64
	WindowID        string
	MaxReservations int
}

// Reservation authorizes one provider invocation only when AlreadyReserved is
// false. Duplicate reservations, including settled ones, MUST NOT invoke it.
type Reservation struct {
	JobID           string
	CallID          string
	MaxCostMicros   int64
	AlreadyReserved bool
	AlreadySettled  bool
	ledgerID        string
	key             string
}

type ledgerEntry struct {
	Key     string `json:"key"`
	Maximum int64  `json:"maximum_micros"`
	Actual  int64  `json:"actual_micros"`
	Settled bool   `json:"settled"`
}

type ledgerState struct {
	Version         int           `json:"version"`
	ID              string        `json:"id"`
	WindowID        string        `json:"window_id"`
	MaxWindowMicros int64         `json:"max_window_micros"`
	MaxReservations int           `json:"max_reservations"`
	UpdatedAt       string        `json:"updated_at"`
	Blocked         bool          `json:"blocked"`
	Entries         []ledgerEntry `json:"entries"`
}

// Ledger holds an exclusive process lock until Close. All successful changes
// have been fsynced, atomically replaced, and directory-fsynced before return.
type Ledger struct {
	mu     sync.Mutex
	root   *os.Root
	dir    *os.File
	lock   *os.File
	name   string
	state  ledgerState
	digest [32]byte
	now    func() time.Time
	closed bool
	failed bool
}

func normalizedLimits(limits LedgerLimits) (LedgerLimits, error) {
	if limits.MaxReservations == 0 {
		limits.MaxReservations = 1024
	}
	if limits.MaxWindowMicros <= 0 || !validIdentity(limits.WindowID) || limits.MaxReservations < 1 || limits.MaxReservations > 4096 {
		return limits, fmt.Errorf("%w: invalid limits", ErrLedgerUnavailable)
	}
	return limits, nil
}

func validIdentity(id string) bool {
	if len(id) == 0 || len(id) > 128 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == ':' || r == '.') {
			return false
		}
	}
	return true
}

func reservationKey(jobID, callID string) string {
	sum := sha256.Sum256([]byte(jobID + "\x00" + callID))
	return hex.EncodeToString(sum[:])
}

// InitializeLedger is an explicit operator action for first accounting setup.
// It refuses existing or interrupted state. Rotation/recovery must reconcile
// the prior window and outstanding charges; this API never erases a ledger.
func InitializeLedger(path string, limits LedgerLimits, windowID string) error {
	if limits.WindowID != "" && limits.WindowID != windowID {
		return ErrLedgerUnavailable
	}
	limits.WindowID = windowID
	limits, err := normalizedLimits(limits)
	if err != nil {
		return err
	}
	l, err := openStorage(path, true)
	if err != nil {
		return err
	}
	defer l.Close()
	if _, err := l.root.Lstat(l.name); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: refusing accounting reset", ErrLedgerUnavailable)
	}
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ErrLedgerUnavailable
	}
	state := ledgerState{Version: 1, ID: hex.EncodeToString(id[:]), WindowID: limits.WindowID, MaxWindowMicros: limits.MaxWindowMicros, MaxReservations: limits.MaxReservations, UpdatedAt: time.Now().UTC().Format(time.RFC3339Nano), Entries: []ledgerEntry{}}
	if err := l.writeMarker(state.ID); err != nil {
		return err
	}
	return l.writeState(state)
}

// OpenLedger never initializes missing accounting or resets a spending window.
func OpenLedger(path string, limits LedgerLimits, now func() time.Time) (*Ledger, error) {
	limits, err := normalizedLimits(limits)
	if err != nil {
		return nil, err
	}
	l, err := openStorage(path, false)
	if err != nil {
		return nil, err
	}
	if now == nil {
		now = time.Now
	}
	l.now = now
	state, digest, err := l.readState()
	if err == nil {
		err = validateState(state, limits)
	}
	if err != nil {
		_ = l.Close()
		return nil, err
	}
	l.state, l.digest = state, digest
	if state.Blocked {
		_ = l.Close()
		return nil, ErrLedgerBlocked
	}
	return l, nil
}

// Reserve durably charges the upper bound before the caller invokes a provider.
// An unknown provider outcome must remain reserved, including across restarts.
func (l *Ledger) Reserve(jobID, callID string, maxCostMicros int64) (Reservation, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ready(); err != nil {
		return Reservation{}, err
	}
	if !validIdentity(jobID) || !validIdentity(callID) || maxCostMicros <= 0 {
		return Reservation{}, ErrInvalidReservation
	}
	key := reservationKey(jobID, callID)
	var total int64
	for _, e := range l.state.Entries {
		if e.Key == key {
			if e.Maximum != maxCostMicros {
				return Reservation{}, ErrReservationConflict
			}
			return Reservation{JobID: jobID, CallID: callID, MaxCostMicros: e.Maximum, AlreadyReserved: true, AlreadySettled: e.Settled, ledgerID: l.state.ID, key: key}, nil
		}
		charge := e.Maximum
		if e.Settled {
			charge = e.Actual
		}
		total += charge
	}
	if maxCostMicros > l.state.MaxWindowMicros-total {
		return Reservation{}, ErrBudgetExceeded
	}
	if len(l.state.Entries) >= l.state.MaxReservations {
		return Reservation{}, fmt.Errorf("%w: reservation capacity reached", ErrLedgerUnavailable)
	}
	next := l.state
	next.Entries = append(append([]ledgerEntry{}, l.state.Entries...), ledgerEntry{Key: key, Maximum: maxCostMicros})
	if err := l.persist(next); err != nil {
		return Reservation{}, err
	}
	return Reservation{JobID: jobID, CallID: callID, MaxCostMicros: maxCostMicros, ledgerID: l.state.ID, key: key}, nil
}

// Settle records an independently verified exact provider charge. Never call
// this for cancellation, timeout, transport failure, or unknown completion.
// A reported charge above its reserved bound durably blocks further paid work.
func (l *Ledger) Settle(res Reservation, actualCostMicros int64) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := l.ready(); err != nil {
		return err
	}
	if actualCostMicros < 0 || res.ledgerID != l.state.ID || res.key != reservationKey(res.JobID, res.CallID) {
		return ErrInvalidReservation
	}
	for i, e := range l.state.Entries {
		if e.Key != res.key {
			continue
		}
		if e.Maximum != res.MaxCostMicros {
			return ErrInvalidReservation
		}
		next := l.state
		if actualCostMicros > e.Maximum {
			next.Blocked = true
			if err := l.persist(next); err != nil {
				return err
			}
			return ErrLedgerBlocked
		}
		if e.Settled {
			if e.Actual != actualCostMicros {
				return ErrReservationConflict
			}
			return nil
		}
		next.Entries = append([]ledgerEntry{}, l.state.Entries...)
		next.Entries[i].Settled, next.Entries[i].Actual = true, actualCostMicros
		return l.persist(next)
	}
	return ErrInvalidReservation
}

func (l *Ledger) persist(next ledgerState) error {
	next.UpdatedAt = l.now().UTC().Format(time.RFC3339Nano)
	if err := l.writeState(next); err != nil {
		l.failed = true
		return err
	}
	l.state = next
	return nil
}

func (l *Ledger) ready() error {
	if l.closed || l.failed {
		return ErrLedgerUnavailable
	}
	if l.state.Blocked {
		return ErrLedgerBlocked
	}
	_, digest, err := l.readState()
	if err != nil || digest != l.digest {
		l.failed = true
		return ErrLedgerUnavailable
	}
	return nil
}

func validateState(state ledgerState, limits LedgerLimits) error {
	if state.Version != 1 || len(state.ID) != 32 || state.WindowID != limits.WindowID || state.MaxWindowMicros != limits.MaxWindowMicros || state.MaxReservations != limits.MaxReservations || len(state.Entries) > limits.MaxReservations || state.Entries == nil {
		return ErrLedgerUnavailable
	}
	if _, err := hex.DecodeString(state.ID); err != nil {
		return ErrLedgerUnavailable
	}
	if _, err := time.Parse(time.RFC3339Nano, state.UpdatedAt); err != nil {
		return ErrLedgerUnavailable
	}
	seen := map[string]bool{}
	var total int64
	for _, e := range state.Entries {
		if len(e.Key) != 64 || seen[e.Key] || e.Maximum <= 0 || e.Maximum > limits.MaxWindowMicros || e.Actual < 0 || e.Actual > e.Maximum || (!e.Settled && e.Actual != 0) {
			return ErrLedgerUnavailable
		}
		if _, err := hex.DecodeString(e.Key); err != nil {
			return ErrLedgerUnavailable
		}
		seen[e.Key] = true
		charge := e.Maximum
		if e.Settled {
			charge = e.Actual
		}
		if charge > limits.MaxWindowMicros-total {
			return ErrLedgerUnavailable
		}
		total += charge
	}
	return nil
}
