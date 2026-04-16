// Package testutil is a small collection of helpers shared by txsim's test
// files. Keeping these in internal/testutil means test files stay terse and
// the copy-paste "t.TempDir + store.Open + engine.New" pattern only lives in
// one place.
package testutil

import (
	"encoding/json"
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

// MustCreateCustomer creates a customer and returns it, failing the test on
// error. Handy for seeding fixtures across many tests.
func MustCreateCustomer(t *testing.T, e *engine.Engine, email string) store.Customer {
	t.Helper()
	c, err := e.CreateCustomer(engine.CreateCustomerParams{Email: email, Name: "Test " + email})
	require.NoError(t, err)
	return c
}

// MustCharge creates a successful charge against the customer and returns the
// persisted Charge record.
func MustCharge(t *testing.T, e *engine.Engine, cus string, amount int64) store.Charge {
	t.Helper()
	r, err := e.Charge(engine.ChargeParams{Amount: amount, Currency: "usd", Customer: cus})
	require.NoError(t, err)
	return r.Charge
}

// DecodeJSON unmarshals b into a generic map. Fails the test on error.
func DecodeJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(b, &out))
	return out
}
