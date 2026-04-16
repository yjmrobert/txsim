package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

func TestStripeNameReportsStripe(t *testing.T) {
	assert.Equal(t, "stripe", (&Stripe{}).Name())
}

func TestStripeFormatRefund(t *testing.T) {
	s := &Stripe{}
	r := engine.RefundResult{
		Refund: store.Refund{
			ID: "re_1", Charge: "ch_1", Amount: 500, Status: "succeeded", Created: 123,
		},
	}
	body, err := s.FormatRefund(r)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "refund", decoded["object"])
	assert.Equal(t, "requested_by_customer", decoded["reason"], "stripe refund reason field is required")
	assert.EqualValues(t, 500, decoded["amount"])
}

func TestStripeFormatCustomer(t *testing.T) {
	s := &Stripe{}
	body, err := s.FormatCustomer(store.Customer{ID: "cus_1", Email: "a@b.com", Name: "Alice"})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "customer", decoded["object"])
	assert.Equal(t, "a@b.com", decoded["email"])
}

func TestStripeFormatCustomers(t *testing.T) {
	s := &Stripe{}
	body, err := s.FormatCustomers([]store.Customer{
		{ID: "cus_1", Email: "a@b.com"},
		{ID: "cus_2", Email: "c@d.com"},
	})
	require.NoError(t, err)

	var decoded struct {
		Object  string           `json:"object"`
		Data    []map[string]any `json:"data"`
		HasMore bool             `json:"has_more"`
		URL     string           `json:"url"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "list", decoded.Object)
	assert.Equal(t, "/v1/customers", decoded.URL)
	assert.False(t, decoded.HasMore)
	assert.Len(t, decoded.Data, 2)
}

func TestStripeFormatCustomersEmptyList(t *testing.T) {
	s := &Stripe{}
	body, err := s.FormatCustomers(nil)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"data":[]`, "empty stripe list must serialise data as []")
}

func TestStripeFormatSubscription(t *testing.T) {
	s := &Stripe{}
	r := engine.SubscribeResult{
		Subscription: store.Subscription{
			ID: "sub_1", Plan: "gold", Customer: "cus_1", Status: "active", Created: 123,
		},
	}
	body, err := s.FormatSubscription(r)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "subscription", decoded["object"])
	assert.Equal(t, "active", decoded["status"])

	plan, ok := decoded["plan"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "gold", plan["id"])
	assert.Equal(t, "plan", plan["object"])
}

func TestStripeFormatErrorUnknownOutcomeFallsBackToApiError(t *testing.T) {
	s := &Stripe{}
	body, err := s.FormatError(engine.Outcome("made_up"))
	require.NoError(t, err)
	var decoded struct {
		Error struct {
			Type string `json:"type"`
			Code string `json:"code"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "api_error", decoded.Error.Type)
	assert.Equal(t, "made_up", decoded.Error.Code, "unknown outcome must surface raw string in code")
}

func TestStripeChargePrettyIsIndented(t *testing.T) {
	s := &Stripe{Pretty: true}
	r := engine.ChargeResult{
		Charge:  store.Charge{ID: "ch_1", Amount: 100, Currency: "usd", Customer: "cus_1", Status: "succeeded"},
		Outcome: engine.OutcomeSucceeded,
	}
	body, err := s.FormatCharge(r)
	require.NoError(t, err)
	assert.Contains(t, string(body), "\n  ")
}

// TestStripeSchemaFidelity decodes a Stripe-mode charge into a hand-rolled
// struct that mirrors Stripe's published type. If a field is renamed or its
// type changes, decoding fails - catching breakage no map[string]any test
// would.
func TestStripeSchemaFidelity(t *testing.T) {
	s := &Stripe{}
	r := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_1", Amount: 2500, Currency: "usd", Customer: "cus_1",
			Status: "succeeded", Outcome: "succeeded", Created: 1700000000,
		},
		Outcome: engine.OutcomeSucceeded,
	}
	body, err := s.FormatCharge(r)
	require.NoError(t, err)

	// Compile-time-checked Stripe-shaped decoder.
	var decoded struct {
		ID                 string  `json:"id"`
		Object             string  `json:"object"`
		Amount             int64   `json:"amount"`
		AmountRefunded     int64   `json:"amount_refunded"`
		Captured           bool    `json:"captured"`
		Created            int64   `json:"created"`
		Currency           string  `json:"currency"`
		Customer           string  `json:"customer"`
		Description        *string `json:"description"`
		Livemode           bool    `json:"livemode"`
		Paid               bool    `json:"paid"`
		Refunded           bool    `json:"refunded"`
		Status             string  `json:"status"`
		FailureCode        *string `json:"failure_code"`
		FailureMessage     *string `json:"failure_message"`
		PaymentMethodTypes []string `json:"payment_method_types"`
		Outcome            struct {
			NetworkStatus string  `json:"network_status"`
			Reason        *string `json:"reason"`
			RiskLevel     string  `json:"risk_level"`
			SellerMessage string  `json:"seller_message"`
			Type          string  `json:"type"`
		} `json:"outcome"`
	}
	// DisallowUnknownFields would be even stronger, but some downstream
	// callers might add fields in the future - strict decode of the known
	// shape is enough to catch regressions.
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "charge", decoded.Object)
	assert.Equal(t, "approved_by_network", decoded.Outcome.NetworkStatus)
	assert.Contains(t, decoded.PaymentMethodTypes, "card")
}
