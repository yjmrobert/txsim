package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

func TestStripeChargeSucceeded(t *testing.T) {
	s := &Stripe{}
	result := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_test", Amount: 2500, Currency: "usd", Customer: "cus_test",
			Status: "succeeded", Outcome: "succeeded", Created: 1700000000,
		},
		Outcome: engine.OutcomeSucceeded,
	}
	body, err := s.FormatCharge(result)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))

	assert.Equal(t, "charge", decoded["object"])
	assert.Equal(t, "succeeded", decoded["status"])
	assert.Equal(t, "cus_test", decoded["customer"])
	assert.EqualValues(t, 2500, decoded["amount"])
	assert.Equal(t, true, decoded["paid"])

	outcome, ok := decoded["outcome"].(map[string]any)
	require.True(t, ok, "expected outcome object")
	assert.Equal(t, "approved_by_network", outcome["network_status"])
}

func TestStripeChargeDeclinedShapesFailureCode(t *testing.T) {
	s := &Stripe{}
	result := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_test", Amount: 500, Currency: "usd", Customer: "cus_test",
			Status: "failed", Outcome: "card_declined", FailureMsg: "Your card was declined.",
		},
		Outcome: engine.OutcomeCardDeclined,
	}
	body, err := s.FormatCharge(result)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))

	assert.Equal(t, "failed", decoded["status"])
	assert.Equal(t, "card_declined", decoded["failure_code"])
	assert.Equal(t, false, decoded["paid"])

	outcome := decoded["outcome"].(map[string]any)
	assert.Equal(t, "declined_by_network", outcome["network_status"])
	assert.Equal(t, "issuer_declined", outcome["type"])
}

func TestStripeErrorEnvelopes(t *testing.T) {
	s := &Stripe{}

	cases := []struct {
		outcome  engine.Outcome
		wantType string
		wantCode string
		wantDecl string
	}{
		{engine.OutcomeCardDeclined, "card_error", "card_declined", ""},
		{engine.OutcomeInsufficient, "card_error", "card_declined", "insufficient_funds"},
		{engine.OutcomeExpiredCard, "card_error", "expired_card", ""},
		{engine.OutcomeProcessingError, "api_error", "processing_error", ""},
		{engine.OutcomeRateLimited, "rate_limit_error", "rate_limit", ""},
	}

	for _, tc := range cases {
		t.Run(string(tc.outcome), func(t *testing.T) {
			body, err := s.FormatError(tc.outcome)
			require.NoError(t, err)
			var decoded struct {
				Error struct {
					Type        string `json:"type"`
					Code        string `json:"code"`
					DeclineCode string `json:"decline_code"`
					Message     string `json:"message"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal(body, &decoded))
			assert.Equal(t, tc.wantType, decoded.Error.Type)
			assert.Equal(t, tc.wantCode, decoded.Error.Code)
			assert.Equal(t, tc.wantDecl, decoded.Error.DeclineCode)
			assert.NotEmpty(t, decoded.Error.Message)
		})
	}
}

func TestGenericChargeShape(t *testing.T) {
	g := &Generic{}
	result := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_test", Amount: 100, Currency: "usd", Customer: "cus_test",
			Status: "succeeded", Outcome: "succeeded",
		},
		Outcome: engine.OutcomeSucceeded,
	}
	body, err := g.FormatCharge(result)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "charge", decoded["type"])
	assert.Equal(t, "succeeded", decoded["status"])
	// Generic shape must NOT contain Stripe-specific fields.
	_, hasObject := decoded["object"]
	_, hasOutcome := decoded["outcome"].(map[string]any)
	assert.False(t, hasObject)
	assert.False(t, hasOutcome)
}

func TestGetUnknownMode(t *testing.T) {
	_, err := Get("paypal", false)
	require.Error(t, err)
}
