package provider

import (
	"testing"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// BenchmarkStripeFormatCharge measures pure marshalling cost of a single
// Stripe charge. MockPay pitches itself as low-latency for agent use, so
// tracking this number across changes gives us early warning of regressions.
func BenchmarkStripeFormatCharge(b *testing.B) {
	s := &Stripe{}
	r := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_bench", Amount: 2500, Currency: "usd", Customer: "cus_bench",
			Status: "succeeded", Outcome: "succeeded", Created: 1700000000,
		},
		Outcome: engine.OutcomeSucceeded,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.FormatCharge(r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenericFormatCharge(b *testing.B) {
	g := &Generic{}
	r := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_bench", Amount: 2500, Currency: "usd", Customer: "cus_bench",
			Status: "succeeded", Outcome: "succeeded",
		},
		Outcome: engine.OutcomeSucceeded,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := g.FormatCharge(r); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStripeFormatError(b *testing.B) {
	s := &Stripe{}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.FormatError(engine.OutcomeInsufficient); err != nil {
			b.Fatal(err)
		}
	}
}
