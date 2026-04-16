package provider

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// updateGolden lets contributors regenerate every fixture with
//
//	go test ./internal/provider -run TestGolden -update
//
// It deliberately re-uses a flag name, not an env var, so it shows up in
// -h output and can be passed alongside -run.
var updateGolden = flag.Bool("update", false, "rewrite provider testdata/*.json golden files")

// goldenCase is one scenario rendered by every provider.
type goldenCase struct {
	name string
	// fn renders the case against a Formatter and returns its JSON bytes.
	fn func(Formatter) ([]byte, error)
}

// fixedCharge / fixedRefund / etc. are stable inputs so diffing generated
// JSON against golden files catches real shape drift rather than timestamp or
// ID churn.
var (
	fixedCustomer = store.Customer{ID: "cus_golden", Email: "golden@example.com", Name: "Golden", Created: 1700000000}
	fixedCharge   = store.Charge{
		ID: "ch_golden", Amount: 2500, Currency: "usd", Customer: fixedCustomer.ID,
		Status: "succeeded", Outcome: "succeeded", Created: 1700000000,
	}
	fixedChargeDeclined = store.Charge{
		ID: "ch_declined", Amount: 500, Currency: "usd", Customer: fixedCustomer.ID,
		Status: "failed", Outcome: string(engine.OutcomeCardDeclined),
		FailureMsg: engine.OutcomeCardDeclined.Message(),
		Created:    1700000000,
	}
	fixedRefund = store.Refund{
		ID: "re_golden", Charge: fixedCharge.ID, Amount: 1000, Status: "succeeded", Created: 1700000000,
	}
	fixedSub = store.Subscription{
		ID: "sub_golden", Plan: "gold", Customer: fixedCustomer.ID, Status: "active", Created: 1700000000,
	}
)

// cases returns the full list of scenarios rendered for every provider.
// Adding a new file shape is as simple as appending here and running
// `go test -run TestGolden -update`.
func cases() []goldenCase {
	return []goldenCase{
		{"charge_succeeded", func(f Formatter) ([]byte, error) {
			return f.FormatCharge(engine.ChargeResult{Charge: fixedCharge, Outcome: engine.OutcomeSucceeded})
		}},
		{"charge_declined", func(f Formatter) ([]byte, error) {
			return f.FormatCharge(engine.ChargeResult{Charge: fixedChargeDeclined, Outcome: engine.OutcomeCardDeclined})
		}},
		{"refund", func(f Formatter) ([]byte, error) {
			return f.FormatRefund(engine.RefundResult{Refund: fixedRefund, Outcome: engine.OutcomeSucceeded})
		}},
		{"customer", func(f Formatter) ([]byte, error) {
			return f.FormatCustomer(fixedCustomer)
		}},
		{"customer_list", func(f Formatter) ([]byte, error) {
			return f.FormatCustomers([]store.Customer{fixedCustomer})
		}},
		{"subscription", func(f Formatter) ([]byte, error) {
			return f.FormatSubscription(engine.SubscribeResult{Subscription: fixedSub, Outcome: engine.OutcomeSucceeded})
		}},
		{"error_card_declined", func(f Formatter) ([]byte, error) {
			return f.FormatError(engine.OutcomeCardDeclined)
		}},
		{"error_insufficient_funds", func(f Formatter) ([]byte, error) {
			return f.FormatError(engine.OutcomeInsufficient)
		}},
		{"error_expired_card", func(f Formatter) ([]byte, error) {
			return f.FormatError(engine.OutcomeExpiredCard)
		}},
		{"error_processing_error", func(f Formatter) ([]byte, error) {
			return f.FormatError(engine.OutcomeProcessingError)
		}},
		{"error_rate_limited", func(f Formatter) ([]byte, error) {
			return f.FormatError(engine.OutcomeRateLimited)
		}},
	}
}

func providers() []Formatter {
	// Pretty is forced so golden files are diff-friendly. The compact form
	// is implicitly covered: it's the same Marshal path with a different
	// indent setting.
	return []Formatter{&Generic{Pretty: true}, &Stripe{Pretty: true}}
}

// TestGolden is the snapshot regression net. For every (provider, case)
// combination it renders the JSON, normalises it, and compares with the
// checked-in fixture under testdata/<provider>/<case>.json.
func TestGolden(t *testing.T) {
	for _, p := range providers() {
		for _, tc := range cases() {
			p, tc := p, tc
			t.Run(p.Name()+"/"+tc.name, func(t *testing.T) {
				got, err := tc.fn(p)
				require.NoError(t, err)
				got = normaliseJSON(t, got)

				path := filepath.Join("testdata", p.Name(), tc.name+".json")
				if *updateGolden {
					require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
					require.NoError(t, os.WriteFile(path, got, 0o644))
					return
				}
				want, err := os.ReadFile(path)
				require.NoErrorf(t, err, "missing golden file %s — run `go test -run TestGolden -update`", path)
				if !bytes.Equal(bytes.TrimSpace(got), bytes.TrimSpace(want)) {
					t.Fatalf("golden mismatch for %s\n--- got ---\n%s\n--- want ---\n%s",
						path, string(got), string(want))
				}
			})
		}
	}
}

// normaliseJSON rewrites the bytes through encoding/json with stable indent
// so trailing newlines and key ordering don't flap between runs.
func normaliseJSON(t *testing.T, b []byte) []byte {
	t.Helper()
	var v any
	require.NoError(t, json.Unmarshal(b, &v))
	out, err := json.MarshalIndent(v, "", "  ")
	require.NoError(t, err)
	return append(out, '\n')
}
