package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yjmrobert/txsim/internal/store"
)

// FuzzStateLoad feeds arbitrary bytes into Store.Load via a state file.
// Invariant: Load must never panic — it either returns a valid *State or a
// clean error.
func FuzzStateLoad(f *testing.F) {
	// Seed corpus: the minimum valid state, a fully populated one, and some
	// near-miss inputs we've already seen catch bugs.
	f.Add([]byte(`{"version":1}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(``))
	f.Add([]byte(`{"version":1,"customers":{"cus_1":{"id":"cus_1","email":"a"}}}`))
	f.Add([]byte(`{"version":1,"customers":null}`))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"version":"not-a-number"}`)) // wrong type

	f.Fuzz(func(t *testing.T, payload []byte) {
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			t.Fatalf("seed write: %v", err)
		}
		s, err := store.Open(path)
		if err != nil {
			t.Fatalf("Open should succeed regardless of file contents: %v", err)
		}
		// Don't care whether it succeeds or errors — just must not panic.
		_, _ = s.Load()
	})
}
