package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

var ErrNoValidState = errors.New("no valid state generation")

type Source string

const (
	SourceCurrent  Source = "current"
	SourcePrevious Source = "previous"
)

type Store struct {
	dir string
}

func NewStore(dir string) *Store { return &Store{dir: dir} }

func (s *Store) currentPath() string  { return filepath.Join(s.dir, "current.json") }
func (s *Store) previousPath() string { return filepath.Join(s.dir, "previous.json") }
func (s *Store) lockPath() string     { return filepath.Join(s.dir, "lock") }

func (s *Store) Ensure() error {
	return os.MkdirAll(s.dir, 0o700)
}

func (s *Store) Load() (State, Source, error) {
	if st, err := readAndValidate(s.currentPath()); err == nil {
		return st, SourceCurrent, nil
	}
	if st, err := readAndValidate(s.previousPath()); err == nil {
		return st, SourcePrevious, nil
	}
	return State{}, "", ErrNoValidState
}

func (s *Store) Write(next State) (State, error) {
	if err := s.Ensure(); err != nil {
		return State{}, fmt.Errorf("create state dir: %w", err)
	}

	lock, err := os.OpenFile(s.lockPath(), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return State{}, fmt.Errorf("open state lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return State{}, fmt.Errorf("lock state: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN) //nolint:errcheck

	var baseSeq uint64
	if current, _, err := s.Load(); err == nil {
		baseSeq = current.Sequence
	}
	next.Sequence = baseSeq + 1
	if next.StateVersion == 0 {
		next.StateVersion = Version
	}
	if err := validateSemantic(next); err != nil {
		return State{}, err
	}

	payload, err := withChecksum(next)
	if err != nil {
		return State{}, err
	}

	tmp, err := os.CreateTemp(s.dir, ".state-*.tmp")
	if err != nil {
		return State{}, fmt.Errorf("create temp state: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		tmp.Close()        //nolint:errcheck
		os.Remove(tmpName) //nolint:errcheck
	}
	defer cleanup()
	if err := tmp.Chmod(0o600); err != nil {
		return State{}, fmt.Errorf("chmod temp state: %w", err)
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		return State{}, fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return State{}, fmt.Errorf("fsync temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return State{}, fmt.Errorf("close temp state: %w", err)
	}

	// Only rotate a checksum- and semantics-valid current generation. If the
	// current file is corrupt and Load() fell back to previous.json, preserving
	// previous.json is more important than retaining the corrupt current file.
	// This guarantees that a valid fallback generation is not destroyed before
	// the new current generation is atomically installed.
	if _, err := readAndValidate(s.currentPath()); err == nil {
		// On Linux/POSIX, rename atomically replaces an existing destination.
		// This avoids an explicit unlink window where no fallback exists.
		if err := os.Rename(s.currentPath(), s.previousPath()); err != nil {
			return State{}, fmt.Errorf("rotate current state: %w", err)
		}
		// Make the valid fallback rename durable before touching current.json.
		if err := syncDir(s.dir); err != nil {
			return State{}, fmt.Errorf("fsync rotated state dir: %w", err)
		}
	}

	// Rename atomically replaces a corrupt current.json if one exists. When the
	// old current was invalid, previous.json has intentionally been left intact.
	if err := os.Rename(tmpName, s.currentPath()); err != nil {
		return State{}, fmt.Errorf("activate current state: %w", err)
	}
	if err := syncDir(s.dir); err != nil {
		return State{}, fmt.Errorf("fsync state dir: %w", err)
	}
	return payload, nil
}

func readAndValidate(path string) (State, error) {
	f, err := os.Open(path)
	if err != nil {
		return State{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, err
	}
	if err := validateSemantic(st); err != nil {
		return State{}, err
	}
	if st.Checksum == "" {
		return State{}, errors.New("state checksum missing")
	}
	expected, err := checksum(st)
	if err != nil {
		return State{}, err
	}
	if st.Checksum != expected {
		return State{}, errors.New("state checksum mismatch")
	}
	return st, nil
}

func validateSemantic(st State) error {
	if st.StateVersion != Version {
		return fmt.Errorf("unsupported state version %d", st.StateVersion)
	}
	if st.TransactionID == "" {
		return errors.New("transaction_id is required")
	}
	if st.ActiveConfigRevision == "" {
		return errors.New("active_config_revision is required")
	}
	if st.Hosts == nil {
		return errors.New("hosts map is required")
	}
	if st.RecoveryStarted && !st.ShutdownCommitted {
		return errors.New("recovery_started requires shutdown_committed")
	}
	if st.PowerState == RestoreHosts && !st.RecoveryStarted {
		return errors.New("RESTORE_HOSTS requires recovery_started")
	}
	return nil
}

func withChecksum(st State) (State, error) {
	st.Checksum = ""
	sum, err := checksum(st)
	if err != nil {
		return State{}, err
	}
	st.Checksum = sum
	return st, nil
}

func checksum(st State) (string, error) {
	st.Checksum = ""
	b, err := json.Marshal(st)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func syncDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
