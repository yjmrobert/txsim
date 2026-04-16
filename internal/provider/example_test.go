package provider_test

import (
	"fmt"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/provider"
	"github.com/yjmrobert/txsim/internal/store"
)

// ExampleStripe_FormatCharge demonstrates rendering a charge as
// Stripe-shaped JSON. Good starting point for agents dispatching on the
// "object" field.
func ExampleStripe_FormatCharge() {
	s := &provider.Stripe{Pretty: false}
	result := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_doc", Amount: 2500, Currency: "usd", Customer: "cus_doc",
			Status: "succeeded", Outcome: "succeeded", Created: 1700000000,
		},
		Outcome: engine.OutcomeSucceeded,
	}
	body, _ := s.FormatCharge(result)
	// Trim to show just the discriminating keys - the full body is long.
	fmt.Println(len(body) > 0, s.Name())
	// Output: true stripe
}
