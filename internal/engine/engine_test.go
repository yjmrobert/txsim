package engine

import (
	"errors"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/store"
)

// newTestEngine returns an Engine backed by a fresh temp-directory store with
// a deterministic clock. It also swaps the latency Sleeper for a no-op so
// tests stay fast regardless of --delay values.
func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.json"))
	require.NoError(t, err)
	e := New(s)
	e.Now = func() time.Time { return time.Unix(1700000000, 0) }
	Sleeper = func(time.Duration) {}
	return e
}

func TestCreateCustomerPersistsAndReturnsID(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.CreateCustomer(CreateCustomerParams{Email: "a@b.com", Name: "Alice"})
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(c.ID, "cus_"), "ID should use cus_ prefix")
	assert.Equal(t, int64(1700000000), c.Created)

	got, err := e.GetCustomer(c.ID)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", got.Email)
}

func TestCreateCustomerRequiresEmail(t *testing.T) {
	e := newTestEngine(t)
	_, err := e.CreateCustomer(CreateCustomerParams{})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidInput))
}

func TestChargeHappyPath(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.CreateCustomer(CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)

	r, err := e.Charge(ChargeParams{Amount: 2500, Currency: "usd", Customer: c.ID})
	require.NoError(t, err)
	assert.Equal(t, "succeeded", r.Charge.Status)
	assert.Equal(t, OutcomeSucceeded, r.Outcome)
	assert.True(t, strings.HasPrefix(r.Charge.ID, "ch_"))
}

func TestChargeDeterministicFailurePersists(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.CreateCustomer(CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)

	r, err := e.Charge(ChargeParams{
		Amount:   100,
		Currency: "usd",
		Customer: c.ID,
		Outcome:  OutcomeCardDeclined,
	})
	require.NoError(t, err)
	assert.Equal(t, "failed", r.Charge.Status)
	assert.Equal(t, OutcomeCardDeclined, r.Outcome)
	assert.NotEmpty(t, r.Charge.FailureMsg)
}

func TestChargeUnknownCustomer(t *testing.T) {
	e := newTestEngine(t)
	_, err := e.Charge(ChargeParams{Amount: 1, Currency: "usd", Customer: "cus_missing"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrNotFound))
}

func TestRefundFullAndPartial(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.CreateCustomer(CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)
	ch, err := e.Charge(ChargeParams{Amount: 1000, Currency: "usd", Customer: c.ID})
	require.NoError(t, err)

	// Partial refund
	r1, err := e.Refund(RefundParams{Charge: ch.Charge.ID, Amount: 400})
	require.NoError(t, err)
	assert.EqualValues(t, 400, r1.Refund.Amount)

	// Full refund of the remaining 600
	r2, err := e.Refund(RefundParams{Charge: ch.Charge.ID})
	require.NoError(t, err)
	assert.EqualValues(t, 600, r2.Refund.Amount)

	// Over-refund should fail
	_, err = e.Refund(RefundParams{Charge: ch.Charge.ID, Amount: 1})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidInput))
}

func TestRefundNonRefundableCharge(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.CreateCustomer(CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)
	ch, err := e.Charge(ChargeParams{Amount: 100, Currency: "usd", Customer: c.ID, Outcome: OutcomeCardDeclined})
	require.NoError(t, err)

	_, err = e.Refund(RefundParams{Charge: ch.Charge.ID})
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrInvalidInput))
}

func TestSubscribeHappyAndPastDue(t *testing.T) {
	e := newTestEngine(t)
	c, err := e.CreateCustomer(CreateCustomerParams{Email: "a@b.com"})
	require.NoError(t, err)

	ok, err := e.Subscribe(SubscribeParams{Plan: "gold", Customer: c.ID})
	require.NoError(t, err)
	assert.Equal(t, "active", ok.Subscription.Status)

	bad, err := e.Subscribe(SubscribeParams{Plan: "gold", Customer: c.ID, Outcome: OutcomeInsufficient})
	require.NoError(t, err)
	assert.Equal(t, "past_due", bad.Subscription.Status)
}

func TestParseOutcomeUnknown(t *testing.T) {
	_, err := ParseOutcome("nope")
	require.Error(t, err)
}

func TestDelaySleeperIsInvoked(t *testing.T) {
	var called int32
	old := Sleeper
	Sleeper = func(time.Duration) { atomic.AddInt32(&called, 1) }
	defer func() { Sleeper = old }()

	Delay(100)
	assert.EqualValues(t, 1, atomic.LoadInt32(&called))

	Delay(0) // should be a no-op
	assert.EqualValues(t, 1, atomic.LoadInt32(&called))
}

func TestNewIDPrefixes(t *testing.T) {
	for _, prefix := range []string{"cus", "ch", "re", "sub"} {
		id := NewID(prefix)
		assert.True(t, strings.HasPrefix(id, prefix+"_"), "expected prefix for %q", prefix)
		assert.Len(t, id, len(prefix)+1+idLen)
	}
}
