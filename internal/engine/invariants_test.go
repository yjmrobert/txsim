package engine_test

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
)

// TestInvariantIDsAreUnique draws a large number of IDs with each prefix and
// checks none collide. Not exhaustive (we only sample 16 hex chars), but 10k
// draws catches a broken RNG faster than waiting for a user to hit it.
func TestInvariantIDsAreUnique(t *testing.T) {
	for _, prefix := range []string{"cus", "ch", "re", "sub"} {
		seen := make(map[string]struct{}, 10_000)
		for i := 0; i < 10_000; i++ {
			id := engine.NewID(prefix)
			require.True(t, strings.HasPrefix(id, prefix+"_"))
			if _, dup := seen[id]; dup {
				t.Fatalf("collision at draw %d for prefix %q: %s", i, prefix, id)
			}
			seen[id] = struct{}{}
		}
	}
}

// TestInvariantAllOutcomesRoundTrip asserts every declared Outcome parses
// back from its string form.
func TestInvariantAllOutcomesRoundTrip(t *testing.T) {
	for _, o := range engine.AllOutcomes {
		back, err := engine.ParseOutcome(string(o))
		require.NoError(t, err)
		assert.Equal(t, o, back)
	}
}

// TestInvariantRefundsNeverExceedChargeRandomised is a randomised property
// check. Seeds the RNG so failures are reproducible.
func TestInvariantRefundsNeverExceedChargeRandomised(t *testing.T) {
	r := rand.New(rand.NewSource(0xC0FFEE))

	for run := 0; run < 50; run++ {
		e := newEngine(t)
		c, err := e.CreateCustomer(engine.CreateCustomerParams{Email: "a@b.com"})
		require.NoError(t, err)

		amount := int64(r.Intn(10_000) + 100)
		ch, err := e.Charge(engine.ChargeParams{Amount: amount, Currency: "usd", Customer: c.ID})
		require.NoError(t, err)

		var refunded int64
		// Issue up to 20 partial refunds of random size; every successful
		// refund must keep the running total ≤ original amount.
		for i := 0; i < 20; i++ {
			// Deliberately includes "too large" values to exercise both
			// accept and reject branches.
			want := int64(r.Intn(2000) + 1)
			result, err := e.Refund(engine.RefundParams{Charge: ch.Charge.ID, Amount: want})
			if err != nil {
				continue
			}
			refunded += result.Refund.Amount
			assert.LessOrEqualf(t, refunded, amount,
				"run=%d i=%d: refunds %d exceeded charge %d", run, i, refunded, amount)
		}
	}
}

// TestInvariantListCountMatchesCreates checks that after N creates and M
// calls to Reset, the customer list length equals N (when no reset happened
// after the last create) or 0.
func TestInvariantListCountMatchesCreates(t *testing.T) {
	e := newEngine(t)

	for i := 0; i < 7; i++ {
		_, err := e.CreateCustomer(engine.CreateCustomerParams{Email: "a@b.com"})
		require.NoError(t, err)
	}
	cs, err := e.ListCustomers()
	require.NoError(t, err)
	assert.Len(t, cs, 7)

	require.NoError(t, e.Reset())

	cs, err = e.ListCustomers()
	require.NoError(t, err)
	assert.Empty(t, cs, "Reset must leave the customer list empty")
}
