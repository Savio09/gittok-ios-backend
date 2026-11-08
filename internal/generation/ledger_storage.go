package generation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

const maxLedgerBytes = 2 << 20
const maxMarkerBytes = 256

type ledgerMarker struct {
	Version  int    `json:"version"`
	LedgerID string `json:"ledger_id"`
}

type ledgerEnvelope struct {
	State    json.RawMessage `json:"state"`
	Checksum string          `json:"sha256"`
}

func openStorage(path string, initialize bool) (*Ledger, error) {
	if !filepath.IsAbs(path) || filepath.Base(path) == "." || filepath.Base(path) == string(filepath.Separator) {
		return nil, ErrLedgerUnavailable
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || !parentInfo.IsDir() {
		return nil, ErrLedgerUnavailable
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return nil, ErrLedgerUnavailable
	}
	l := &Ledger{root: root, name: filepath.Base(path)}
	fail := func(err error) (*Ledger, error) { _ = l.Close(); return nil, err }
	l.dir, err = root.Open(".")
	if err != nil {
		return fail(ErrLedgerUnavailable)
	}
	info, err := l.dir.Stat()
	if err != nil || !os.SameFile(parentInfo, info) {
		return fail(ErrLedgerUnavailable)
	}
	if err := privateDirectory(info); err != nil {
		return fail(err)
	}
	flags := os.O_RDWR | syscall.O_NOFOLLOW
	if initialize {
		// The marker distinguishes first setup from lost accounting. Neither
		// startup nor this initializer may recreate only part of a ledger.
		flags |= os.O_CREATE | os.O_EXCL
	}
	l.lock, err = root.OpenFile(l.name+".lock", flags, 0600)
	if err != nil {
		return fail(ErrLedgerUnavailable)
	}
	if err := privateRegularFile(l.lock); err != nil {
		return fail(err)
	}
	if err := syscall.Flock(int(l.lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return fail(ErrLedgerLocked)
		}
		return fail(ErrLedgerUnavailable)
	}
	if _, err := root.Lstat(l.name + ".pending"); !errors.Is(err, os.ErrNotExist) {
		return fail(ErrLedgerUnavailable)
	}
	return l, nil
}

func privateDirectory(info os.FileInfo) error {
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return ErrLedgerUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return ErrLedgerUnavailable
	}
	return nil
}

func privateRegularFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return ErrLedgerUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Geteuid()) {
		return ErrLedgerUnavailable
	}
	return nil
}

func (l *Ledger) checkLock() error {
	dirInfo, err := l.root.Stat(".")
	if err != nil {
		return ErrLedgerUnavailable
	}
	if err := privateDirectory(dirInfo); err != nil {
		return err
	}
	pathInfo, err := l.root.Lstat(l.name + ".lock")
	if err != nil {
		return ErrLedgerUnavailable
	}
	fdInfo, err := l.lock.Stat()
	if err != nil || !os.SameFile(pathInfo, fdInfo) {
		return ErrLedgerUnavailable
	}
	return privateRegularFile(l.lock)
}

// The marker keeps its inode for flock ownership and is written only once,
// before the first snapshot. Interrupted initialization must require recovery.
func (l *Ledger) writeMarker(ledgerID string) error {
	if err := l.checkLock(); err != nil {
		return err
	}
	info, err := l.lock.Stat()
	if err != nil || info.Size() != 0 {
		return ErrLedgerUnavailable
	}
	contents, err := json.Marshal(ledgerMarker{Version: 1, LedgerID: ledgerID})
	if err != nil || len(contents) > maxMarkerBytes {
		return ErrLedgerUnavailable
	}
	n, err := l.lock.WriteAt(contents, 0)
	if err != nil || n != len(contents) {
		return ErrLedgerUnavailable
	}
	if err := l.lock.Sync(); err != nil {
		return ErrLedgerUnavailable
	}
	if err := l.dir.Sync(); err != nil {
		return ErrLedgerUnavailable
	}
	return nil
}

func (l *Ledger) readMarker() (string, error) {
	contents, err := io.ReadAll(io.NewSectionReader(l.lock, 0, maxMarkerBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > maxMarkerBytes {
		return "", ErrLedgerUnavailable
	}
	var marker ledgerMarker
	if err := decodeStrict(contents, &marker); err != nil || marker.Version != 1 || len(marker.LedgerID) != 32 {
		return "", ErrLedgerUnavailable
	}
	if _, err := hex.DecodeString(marker.LedgerID); err != nil {
		return "", ErrLedgerUnavailable
	}
	return marker.LedgerID, nil
}

func (l *Ledger) readState() (ledgerState, [32]byte, error) {
	var state ledgerState
	var digest [32]byte
	if err := l.checkLock(); err != nil {
		return state, digest, err
	}
	markerID, err := l.readMarker()
	if err != nil {
		return state, digest, err
	}
	if _, err := l.root.Lstat(l.name + ".pending"); !errors.Is(err, os.ErrNotExist) {
		return state, digest, ErrLedgerUnavailable
	}
	file, err := l.root.OpenFile(l.name, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return state, digest, ErrLedgerUnavailable
	}
	defer file.Close()
	if err := privateRegularFile(file); err != nil {
		return state, digest, err
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxLedgerBytes+1))
	if err != nil || len(contents) == 0 || len(contents) > maxLedgerBytes {
		return state, digest, ErrLedgerUnavailable
	}
	var envelope ledgerEnvelope
	if err := decodeStrict(contents, &envelope); err != nil {
		return state, digest, ErrLedgerUnavailable
	}
	sum := sha256.Sum256(envelope.State)
	if envelope.Checksum != hex.EncodeToString(sum[:]) {
		return state, digest, ErrLedgerUnavailable
	}
	if err := decodeStrict(envelope.State, &state); err != nil {
		return state, digest, ErrLedgerUnavailable
	}
	if state.ID != markerID {
		return state, digest, ErrLedgerUnavailable
	}
	return state, sha256.Sum256(contents), nil
}

func decodeStrict(contents []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ErrLedgerUnavailable
	}
	return nil
}

func (l *Ledger) writeState(state ledgerState) error {
	if err := l.checkLock(); err != nil {
		return err
	}
	markerID, err := l.readMarker()
	if err != nil || state.ID != markerID {
		return ErrLedgerUnavailable
	}
	payload, err := json.Marshal(state)
	if err != nil {
		return ErrLedgerUnavailable
	}
	sum := sha256.Sum256(payload)
	contents, err := json.Marshal(ledgerEnvelope{State: payload, Checksum: hex.EncodeToString(sum[:])})
	if err != nil || len(contents) > maxLedgerBytes {
		return ErrLedgerUnavailable
	}
	file, err := l.root.OpenFile(l.name+".pending", os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return ErrLedgerUnavailable
	}
	// Leave interrupted writes in place. Startup must not discard evidence of
	// an unacknowledged accounting change or silently initialize fresh state.
	n, writeErr := file.Write(contents)
	if writeErr == nil && n != len(contents) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		return ErrLedgerUnavailable
	}
	if err := l.root.Rename(l.name+".pending", l.name); err != nil {
		return ErrLedgerUnavailable
	}
	if err := l.dir.Sync(); err != nil {
		return ErrLedgerUnavailable
	}
	l.digest = sha256.Sum256(contents)
	return nil
}

func (l *Ledger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return nil
	}
	l.closed = true
	var errs []error
	if l.lock != nil {
		// Closing releases only this descriptor's ownership, including when an
		// attempted second opener failed to acquire the exclusive lock.
		errs = append(errs, l.lock.Close())
	}
	if l.dir != nil {
		errs = append(errs, l.dir.Close())
	}
	if l.root != nil {
		errs = append(errs, l.root.Close())
	}
	return errors.Join(errs...)
}
