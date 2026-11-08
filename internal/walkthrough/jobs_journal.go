package walkthrough

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// The journal stores receipts, not generated content or source. It is bounded
// and process-exclusive. Spending lives in a separate ledger that survives a
// lost journal; a lost receipt can never authorize replay of its old calls.
type journalRecord struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Identity  Identity  `json:"identity"`
	Created   time.Time `json:"created"`
	Completed time.Time `json:"completed"`
}
type journalSnapshot struct {
	Version int             `json:"version"`
	Records []journalRecord `json:"records"`
}
type jobJournal struct {
	path string
	lock *os.File
	max  int
}

func openJobJournal(path string, max int) (*jobJournal, map[string]*jobRecord, error) {
	fail := func() (*jobJournal, map[string]*jobRecord, error) { return nil, nil, ErrJobUnavailable }
	if path == "" || !filepath.IsAbs(path) {
		return fail()
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fail()
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0077 != 0 {
		return fail()
	}
	if stat, ok := info.Sys().(*syscall.Stat_t); !ok || stat.Uid != uint32(os.Getuid()) {
		return fail()
	}
	// Refuse an indirect parent path too; no symlink can redirect control state.
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fail()
	}
	// Canonicalize trusted system aliases such as macOS /var -> /private/var;
	// the dedicated immediate parent above must itself be a real private dir.
	path = filepath.Join(resolved, filepath.Base(path))
	lock, err := openJournalFile(path+".lock", os.O_RDWR|os.O_CREATE)
	if err != nil {
		return fail()
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return fail()
	}
	j := &jobJournal{path: path, lock: lock, max: max}
	records := map[string]*jobRecord{}
	file, err := openJournalFile(path, os.O_RDONLY)
	if errors.Is(err, os.ErrNotExist) {
		return j, records, nil
	}
	if err != nil {
		j.close()
		return fail()
	}
	defer file.Close()
	if info, err := file.Stat(); err != nil || info.Size() > 1<<20 {
		j.close()
		return fail()
	}
	dec := json.NewDecoder(io.LimitReader(file, (1<<20)+1))
	dec.DisallowUnknownFields()
	var snapshot journalSnapshot
	if err := dec.Decode(&snapshot); err != nil || snapshot.Version != 1 || len(snapshot.Records) > max {
		j.close()
		return fail()
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		j.close()
		return fail()
	}
	for _, record := range snapshot.Records {
		_, idErr := hex.DecodeString(record.ID)
		i := record.Identity
		validIdentity := i.RepoID > 0 && ownerPattern.MatchString(i.Owner) && repoPattern.MatchString(i.Name) && i.Name != "." && i.Name != ".." && commitPattern.MatchString(i.Commit)
		if len(record.ID) != 32 || idErr != nil || !validIdentity || record.Created.IsZero() || records[record.ID] != nil || (!activeState(record.State) && record.State != "ready" && record.State != "failed" && record.State != "interrupted" && record.State != "cancelled" && record.State != "unavailable") {
			j.close()
			return fail()
		}
		records[record.ID] = &jobRecord{Job: Job{ID: record.ID, State: record.State}, Identity: record.Identity, Created: record.Created, Completed: record.Completed}
	}
	return j, records, nil
}
func openJournalFile(path string, flags int) (*os.File, error) {
	file, err := os.OpenFile(path, flags|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		file.Close()
		return nil, ErrJobUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Getuid()) || stat.Nlink != 1 {
		file.Close()
		return nil, ErrJobUnavailable
	}
	return file, nil
}
func (j *jobJournal) save(records map[string]*jobRecord) error {
	if j.lock == nil || len(records) > j.max {
		return ErrJobUnavailable
	}
	snapshot := journalSnapshot{Version: 1, Records: make([]journalRecord, 0, len(records))}
	for _, r := range records {
		snapshot.Records = append(snapshot.Records, journalRecord{ID: r.ID, State: r.State, Identity: r.Identity, Created: r.Created, Completed: r.Completed})
	}
	data, err := json.Marshal(snapshot)
	if err != nil || len(data) > 1<<20 {
		return ErrJobUnavailable
	}
	f, err := os.CreateTemp(filepath.Dir(j.path), ".jobs-*")
	if err != nil {
		return err
	}
	temp := f.Name()
	defer os.Remove(temp)
	if err := f.Chmod(0600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if existing, err := os.Lstat(j.path); err == nil {
		if !existing.Mode().IsRegular() || existing.Mode()&os.ModeSymlink != 0 {
			return ErrJobUnavailable
		}
		if stat, ok := existing.Sys().(*syscall.Stat_t); !ok || stat.Nlink != 1 || stat.Uid != uint32(os.Getuid()) {
			return ErrJobUnavailable
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.Rename(temp, j.path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(j.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (j *jobJournal) close() error {
	if j.lock == nil {
		return nil
	}
	_ = syscall.Flock(int(j.lock.Fd()), syscall.LOCK_UN)
	err := j.lock.Close()
	j.lock = nil
	return err
}
