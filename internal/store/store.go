package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

// DefaultPath returns the default state file location: ~/.mockpay/state.json.
// It falls back to ./mockpay-state.json if the home directory can't be resolved.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "mockpay-state.json"
	}
	return filepath.Join(home, ".mockpay", "state.json")
}

// Store is a file-backed State with locking for concurrent safety.
//
// Two layers of locking are used:
//  1. mu is an in-process mutex that serialises goroutines sharing the same
//     Store value. flock on Linux is advisory at the process level and does
//     NOT serialise threads of the same process, so we need this extra mutex.
//  2. lock is a cross-process advisory file lock so multiple mockpay
//     invocations (e.g. CI running several in parallel) can't clobber each
//     other's state.
type Store struct {
	path string
	mu   sync.Mutex
	lock *flock.Flock
}

// Open prepares a Store rooted at path. It does not read the file yet; call
// WithLock to perform an atomic load-mutate-save cycle.
func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	lockPath := path + ".lock"
	return &Store{
		path: path,
		lock: flock.New(lockPath),
	}, nil
}

// Path returns the on-disk state file path.
func (s *Store) Path() string { return s.path }

// Load reads and decodes the state file. Missing file returns a fresh State.
func (s *Store) Load() (*State, error) {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return NewState(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	if len(data) == 0 {
		return NewState(), nil
	}
	st := NewState()
	if err := json.Unmarshal(data, st); err != nil {
		return nil, fmt.Errorf("decode state: %w", err)
	}
	if st.Customers == nil {
		st.Customers = map[string]Customer{}
	}
	if st.Charges == nil {
		st.Charges = map[string]Charge{}
	}
	if st.Refunds == nil {
		st.Refunds = map[string]Refund{}
	}
	if st.Subscriptions == nil {
		st.Subscriptions = map[string]Subscription{}
	}
	return st, nil
}

// Save writes the state to disk atomically (write to temp + rename).
func (s *Store) Save(st *State) error {
	if st.Version == 0 {
		st.Version = StateVersion
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*.json")
	if err != nil {
		return fmt.Errorf("create temp state: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp state: %w", err)
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp state: %w", err)
	}
	return nil
}

// Reset deletes the state file.
func (s *Store) Reset() error {
	err := os.Remove(s.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("reset state: %w", err)
	}
	return nil
}

// WithLock runs fn while holding both the in-process mutex and the
// cross-process file lock. fn receives the current state and may mutate it;
// if fn returns nil, the (mutated) state is saved back.
func (s *Store) WithLock(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.lock.Lock(); err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer s.lock.Unlock()

	st, err := s.Load()
	if err != nil {
		return err
	}
	if err := fn(st); err != nil {
		return err
	}
	return s.Save(st)
}
