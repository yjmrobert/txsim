package provider

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

func TestGenericNameReportsGeneric(t *testing.T) {
	assert.Equal(t, "generic", (&Generic{}).Name())
}

func TestGenericFormatRefund(t *testing.T) {
	g := &Generic{}
	r := engine.RefundResult{
		Refund: store.Refund{
			ID: "re_1", Charge: "ch_1", Amount: 500, Status: "succeeded", Created: 123,
		},
	}
	body, err := g.FormatRefund(r)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "refund", decoded["type"])
	assert.Equal(t, "re_1", decoded["id"])
	assert.Equal(t, "ch_1", decoded["charge"])
	assert.EqualValues(t, 500, decoded["amount"])
	assert.Equal(t, "succeeded", decoded["status"])
}

func TestGenericFormatCustomer(t *testing.T) {
	g := &Generic{}
	body, err := g.FormatCustomer(store.Customer{
		ID: "cus_1", Email: "a@b.com", Name: "Alice", Created: 123,
	})
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "customer", decoded["type"])
	assert.Equal(t, "a@b.com", decoded["email"])
	assert.Equal(t, "Alice", decoded["name"])
}

func TestGenericFormatCustomers(t *testing.T) {
	g := &Generic{}
	body, err := g.FormatCustomers([]store.Customer{
		{ID: "cus_1", Email: "a@b.com", Name: "A"},
		{ID: "cus_2", Email: "c@d.com", Name: "C"},
	})
	require.NoError(t, err)

	var decoded struct {
		Type string           `json:"type"`
		Data []map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "list", decoded.Type)
	assert.Len(t, decoded.Data, 2)
	assert.Equal(t, "a@b.com", decoded.Data[0]["email"])
}

// TestGenericFormatCustomersEmptyIsSerialisedArray asserts that an empty list
// still marshals as "data": [] rather than "data": null. Agents parsing this
// shape need a stable array.
func TestGenericFormatCustomersEmptyIsSerialisedArray(t *testing.T) {
	g := &Generic{}
	body, err := g.FormatCustomers(nil)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"data":[]`)
}

func TestGenericFormatSubscription(t *testing.T) {
	g := &Generic{}
	r := engine.SubscribeResult{
		Subscription: store.Subscription{
			ID: "sub_1", Plan: "gold", Customer: "cus_1", Status: "active", Created: 123,
		},
	}
	body, err := g.FormatSubscription(r)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "subscription", decoded["type"])
	assert.Equal(t, "gold", decoded["plan"])
	assert.Equal(t, "active", decoded["status"])
}

func TestGenericFormatErrorAllOutcomes(t *testing.T) {
	g := &Generic{}
	for _, o := range engine.AllOutcomes {
		if !o.IsFailure() {
			continue
		}
		t.Run(string(o), func(t *testing.T) {
			body, err := g.FormatError(o)
			require.NoError(t, err)
			var decoded map[string]any
			require.NoError(t, json.Unmarshal(body, &decoded))
			assert.Equal(t, "error", decoded["type"])
			assert.Equal(t, string(o), decoded["code"])
			assert.NotEmpty(t, decoded["message"])
		})
	}
}

func TestGenericChargeFailureIncludesMessage(t *testing.T) {
	g := &Generic{}
	r := engine.ChargeResult{
		Charge: store.Charge{
			ID: "ch_1", Amount: 100, Currency: "usd", Customer: "cus_1",
			Status: "failed", Outcome: string(engine.OutcomeCardDeclined),
			FailureMsg: "Your card was declined.",
		},
		Outcome: engine.OutcomeCardDeclined,
	}
	body, err := g.FormatCharge(r)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "failed", decoded["status"])
	assert.NotEmpty(t, decoded["message"], "failure messages must reach the caller")
}

func TestPrettyFlagProducesIndentedOutput(t *testing.T) {
	// The indented form contains newlines; compact does not.
	pretty := &Generic{Pretty: true}
	compact := &Generic{Pretty: false}

	body, err := pretty.FormatCustomer(store.Customer{ID: "cus_1", Email: "a@b.com"})
	require.NoError(t, err)
	assert.Contains(t, string(body), "\n  ", "pretty output should be indented with two spaces")

	body, err = compact.FormatCustomer(store.Customer{ID: "cus_1", Email: "a@b.com"})
	require.NoError(t, err)
	assert.NotContains(t, string(body), "\n", "compact output should be a single line")
}
