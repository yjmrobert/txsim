package store_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/store"
)

func TestDefaultPathEndsInStateJSON(t *testing.T) {
	// Exact path depends on $HOME; we only care that it terminates at
	// .mockpay/state.json and isn't empty.
	p := store.DefaultPath()
	require.NotEmpty(t, p)
	assert.Equal(t, "state.json", filepath.Base(p))
	assert.Equal(t, ".mockpay", filepath.Base(filepath.Dir(p)))
}

func TestDefaultPathFallbackWhenHomeUnresolvable(t *testing.T) {
	// os.UserHomeDir consults $HOME on Unix. Clearing it forces the error
	// branch where DefaultPath falls back to a CWD-relative path.
	if runtime.GOOS == "windows" {
		t.Skip("home-dir env var varies on Windows")
	}
	t.Setenv("HOME", "")
	p := store.DefaultPath()
	assert.Equal(t, "mockpay-state.json", p)
}

func TestOpenWithEmptyPathUsesDefault(t *testing.T) {
	// Ensure passing "" as path doesn't crash - it should resolve to the
	// default. We don't actually touch disk here; Open only creates the
	// parent directory on the caller's behalf.
	// Redirect HOME into a temp dir so we don't pollute the real one.
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	s, err := store.Open("")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmp, ".mockpay", "state.json"), s.Path())
}

func TestLoadRejectsCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0o644))

	s, err := store.Open(path)
	require.NoError(t, err)

	_, err = s.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode state")
}

func TestLoadEmptyFileReturnsFreshState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, nil, 0o644))

	s, err := store.Open(path)
	require.NoError(t, err)

	st, err := s.Load()
	require.NoError(t, err)
	assert.NotNil(t, st.Customers)
	assert.Empty(t, st.Customers)
}

func TestLoadPopulatesMissingMapFields(t *testing.T) {
	// An older state file missing one of the maps should still decode
	// without nil-map panics on subsequent writes.
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"version":1}`), 0o644))

	s, err := store.Open(path)
	require.NoError(t, err)
	st, err := s.Load()
	require.NoError(t, err)
	require.NotNil(t, st.Customers)
	require.NotNil(t, st.Charges)
	require.NotNil(t, st.Refunds)
	require.NotNil(t, st.Subscriptions)

	// Verify we can add to all of them without panicking.
	st.Customers["cus_1"] = store.Customer{ID: "cus_1"}
	st.Charges["ch_1"] = store.Charge{ID: "ch_1"}
	st.Refunds["re_1"] = store.Refund{ID: "re_1"}
	st.Subscriptions["sub_1"] = store.Subscription{ID: "sub_1"}
}

func TestWithLockPropagatesUserError(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "state.json"))
	require.NoError(t, err)

	sentinel := errors.New("boom")
	err = s.WithLock(func(_ *store.State) error { return sentinel })
	require.ErrorIs(t, err, sentinel)
}

func TestWithLockDoesNotPersistOnUserError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := store.Open(path)
	require.NoError(t, err)

	_ = s.WithLock(func(st *store.State) error {
		st.Customers["cus_ghost"] = store.Customer{ID: "cus_ghost"}
		return errors.New("rollback me")
	})

	// No state.json should have been created.
	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "Save must not be called when fn returns an error")
}

func TestPathReturnsConfiguredLocation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.json")
	s, err := store.Open(path)
	require.NoError(t, err)
	assert.Equal(t, path, s.Path())
}

func TestRoundTripPreservesAllFields(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(filepath.Join(dir, "state.json"))
	require.NoError(t, err)

	original := store.NewState()
	original.Customers["cus_1"] = store.Customer{ID: "cus_1", Email: "a@b.com", Name: "Alice", Created: 111}
	original.Charges["ch_1"] = store.Charge{
		ID: "ch_1", Amount: 2500, Currency: "usd", Customer: "cus_1",
		Status: "failed", Outcome: "card_declined", FailureMsg: "nope", Refunded: 0, Created: 222,
	}
	original.Refunds["re_1"] = store.Refund{ID: "re_1", Charge: "ch_1", Amount: 100, Status: "succeeded", Created: 333}
	original.Subscriptions["sub_1"] = store.Subscription{ID: "sub_1", Plan: "gold", Customer: "cus_1", Status: "active", Created: 444}

	require.NoError(t, s.Save(original))
	loaded, err := s.Load()
	require.NoError(t, err)

	// Use JSON equality to sidestep map-pointer inequality.
	a, _ := json.Marshal(original)
	b, _ := json.Marshal(loaded)
	assert.JSONEq(t, string(a), string(b))
}
