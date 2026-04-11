// Package provider formats engine results into provider-specific JSON shapes.
// The interface is deliberately flat so adding new providers (PayPal, crypto,
// internal enterprise systems) is a matter of dropping in a new file and
// registering it.
package provider

import (
	"fmt"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// Formatter renders engine results and failures as provider-shaped JSON.
type Formatter interface {
	// Name is the mode name (e.g. "generic", "stripe").
	Name() string
	FormatCharge(engine.ChargeResult) ([]byte, error)
	FormatRefund(engine.RefundResult) ([]byte, error)
	FormatCustomer(store.Customer) ([]byte, error)
	FormatCustomers([]store.Customer) ([]byte, error)
	FormatSubscription(engine.SubscribeResult) ([]byte, error)
	FormatError(engine.Outcome) ([]byte, error)
}

// Get returns the formatter registered for mode. An unknown mode is a usage
// error.
func Get(mode string, pretty bool) (Formatter, error) {
	switch mode {
	case "", "generic":
		return &Generic{Pretty: pretty}, nil
	case "stripe":
		return &Stripe{Pretty: pretty}, nil
	default:
		return nil, fmt.Errorf("unknown --mode %q (supported: generic, stripe)", mode)
	}
}

// marshal is a small helper that respects the pretty flag while keeping the
// per-provider code uncluttered.
func marshal(v any, pretty bool) ([]byte, error) {
	if pretty {
		return jsonIndent(v)
	}
	return jsonCompact(v)
}
