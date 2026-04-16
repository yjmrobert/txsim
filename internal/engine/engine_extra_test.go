package engine_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/testutil"
)

// newEngine is a thin adapter so existing tests keep reading as "create an
// engine bound to a scratch store".
func newEngine(t *testing.T) *engine.Engine { return testutil.NewEngine(t) }

func TestListCustomersReturnsAll(t *testing.T) {
	e := newEngine(t)

	// Empty store returns empty slice, no error.
	cs, err := e.ListCustomers()
	require.NoError(t, err)
	assert.Empty(t, cs)

	_, err = e.CreateCustomer(engine.CreateCustomerParams{Email: "a@b.com", Name: "Alice"})
	require.NoError(t, err)
	_, err = e.CreateCustomer(engine.CreateCustomerParams{Email: "c@d.com", Name: "Carol"})
	require.NoError(t, err)

	cs, err = e.ListCustomers()
	require.NoError(t, err)
	assert.Len(t, cs, 2)
}

func TestResetRemovesStateFile(t *testing.T) {
	e := newEngine(t)
	_, err := e.CreateCustomer(engine.CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)
	require.FileExists(t, e.Store.Path())

	require.NoError(t, e.Reset())
	_, err = os.Stat(e.Store.Path())
	assert.True(t, os.IsNotExist(err), "state file should be gone after Reset")

	// After reset the engine should still be usable - the store lazily
	// recreates state on the next WithLock call.
	cs, err := e.ListCustomers()
	require.NoError(t, err)
	assert.Empty(t, cs)
}

// TestOutcomeMessageAllCases covers the full switch in Outcome.Message,
// including the fallback path for an unknown outcome value.
func TestOutcomeMessageAllCases(t *testing.T) {
	for _, o := range engine.AllOutcomes {
		t.Run(string(o), func(t *testing.T) {
			assert.NotEmpty(t, o.Message(), "every defined outcome should have a message")
		})
	}
	// Unknown outcome falls back to the raw string.
	unknown := engine.Outcome("wat")
	assert.Equal(t, "wat", unknown.Message())
}

func TestChargeInvalidInputs(t *testing.T) {
	e := newEngine(t)

	cases := []struct {
		name string
		p    engine.ChargeParams
	}{
		{"zero amount", engine.ChargeParams{Currency: "usd", Customer: "cus_x"}},
		{"negative amount", engine.ChargeParams{Amount: -1, Currency: "usd", Customer: "cus_x"}},
		{"no currency", engine.ChargeParams{Amount: 1, Customer: "cus_x"}},
		{"no customer", engine.ChargeParams{Amount: 1, Currency: "usd"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := e.Charge(tc.p)
			require.Error(t, err)
			assert.True(t, errors.Is(err, engine.ErrInvalidInput))
		})
	}
}

func TestRefundFailedOutcomePersistsWithoutReducingCharge(t *testing.T) {
	e := newEngine(t)
	c, err := e.CreateCustomer(engine.CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)
	ch, err := e.Charge(engine.ChargeParams{Amount: 500, Currency: "usd", Customer: c.ID})
	require.NoError(t, err)

	r, err := e.Refund(engine.RefundParams{Charge: ch.Charge.ID, Outcome: engine.OutcomeProcessingError})
	require.NoError(t, err)
	assert.Equal(t, "failed", r.Refund.Status)
	assert.Equal(t, engine.OutcomeProcessingError, r.Outcome)

	// A full refund must still be possible after the failed refund attempt.
	ok, err := e.Refund(engine.RefundParams{Charge: ch.Charge.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 500, ok.Refund.Amount)
}

func TestSubscribeInvalidInputs(t *testing.T) {
	e := newEngine(t)

	_, err := e.Subscribe(engine.SubscribeParams{Customer: "cus_x"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, engine.ErrInvalidInput))

	_, err = e.Subscribe(engine.SubscribeParams{Plan: "gold"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, engine.ErrInvalidInput))

	_, err = e.Subscribe(engine.SubscribeParams{Plan: "gold", Customer: "cus_missing"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, engine.ErrNotFound))
}

func TestGetCustomerUnknown(t *testing.T) {
	e := newEngine(t)
	_, err := e.GetCustomer("cus_missing")
	require.Error(t, err)
	assert.True(t, errors.Is(err, engine.ErrNotFound))
}

// TestInvariantRefundsNeverExceedCharge exercises the "sum(refunds) <=
// charge.amount" invariant across many randomised partial refunds.
func TestInvariantRefundsNeverExceedCharge(t *testing.T) {
	e := newEngine(t)
	c, err := e.CreateCustomer(engine.CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)
	ch, err := e.Charge(engine.ChargeParams{Amount: 10_000, Currency: "usd", Customer: c.ID})
	require.NoError(t, err)

	var refunded int64
	// Refund 100 cents 50 times - exactly exhausts the charge.
	for i := 0; i < 50; i++ {
		r, err := e.Refund(engine.RefundParams{Charge: ch.Charge.ID, Amount: 200})
		require.NoError(t, err)
		refunded += r.Refund.Amount
		assert.LessOrEqual(t, refunded, int64(10_000))
	}

	// The 51st refund must be rejected as over-refund.
	_, err = e.Refund(engine.RefundParams{Charge: ch.Charge.ID, Amount: 1})
	require.Error(t, err)
	assert.True(t, errors.Is(err, engine.ErrInvalidInput))
}
