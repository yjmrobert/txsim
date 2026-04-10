package store

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadMissingFileReturnsEmptyState(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "state.json"))
	require.NoError(t, err)

	st, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, StateVersion, st.Version)
	assert.Empty(t, st.Customers)
	assert.Empty(t, st.Charges)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "state.json"))
	require.NoError(t, err)

	st := NewState()
	st.Customers["cus_1"] = Customer{ID: "cus_1", Email: "a@b.com", Name: "Alice"}
	st.Charges["ch_1"] = Charge{ID: "ch_1", Amount: 2500, Currency: "usd", Customer: "cus_1", Status: "succeeded", Outcome: "succeeded"}

	require.NoError(t, s.Save(st))

	loaded, err := s.Load()
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", loaded.Customers["cus_1"].Email)
	assert.EqualValues(t, 2500, loaded.Charges["ch_1"].Amount)
}

func TestWithLockIsSerialised(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "state.json"))
	require.NoError(t, err)

	// Seed a customer to mutate.
	require.NoError(t, s.WithLock(func(st *State) error {
		st.Customers["cus_1"] = Customer{ID: "cus_1", Email: "a@b.com"}
		return nil
	}))

	// Kick off many concurrent writers that each append a charge. If
	// WithLock isn't properly serialising load→mutate→save, we'll lose
	// updates.
	const writers = 20
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		id := i
		go func() {
			defer wg.Done()
			require.NoError(t, s.WithLock(func(st *State) error {
				st.Charges[chargeID(id)] = Charge{
					ID: chargeID(id), Amount: int64(id + 1), Currency: "usd", Customer: "cus_1", Status: "succeeded",
				}
				return nil
			}))
		}()
	}
	wg.Wait()

	loaded, err := s.Load()
	require.NoError(t, err)
	assert.Len(t, loaded.Charges, writers, "all concurrent writes should land")
}

func TestResetRemovesFile(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "state.json"))
	require.NoError(t, err)

	require.NoError(t, s.Save(NewState()))
	require.NoError(t, s.Reset())
	require.NoError(t, s.Reset(), "reset on missing file should be a no-op")
}

func chargeID(i int) string {
	const hex = "0123456789abcdef"
	// short synthetic ID - avoids reimporting crypto/rand
	return "ch_test_" + string(hex[i%16]) + string(hex[(i/16)%16])
}
