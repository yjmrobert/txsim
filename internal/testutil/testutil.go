// Package testutil is a small collection of helpers shared by txsim's test
// files. Keeping these in internal/testutil means test files stay terse and
// the copy-paste "t.TempDir + store.Open + engine.New" pattern only lives in
// one place.
package testutil

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// NewStore returns a Store backed by a file inside t.TempDir. No state is
// persisted beyond the test's lifetime.
func NewStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)
	return s
}

// NewEngine returns an Engine bound to a fresh temp Store with a fixed clock
// and a no-op Sleeper so tests are fast and deterministic.
func NewEngine(t *testing.T) *engine.Engine {
	t.Helper()
	e := engine.New(NewStore(t))
	e.Now = func() time.Time { return time.Unix(1700000000, 0) }
	engine.Sleeper = func(time.Duration) {}
	return e
}
