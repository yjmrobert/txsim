package provider

import (
	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// Stripe emits JSON shapes that mirror Stripe's real API responses closely
// enough to fool code that dispatches on object types and field names.
type Stripe struct{ Pretty bool }

func (s *Stripe) Name() string { return "stripe" }

// stripeCharge mirrors the subset of https://stripe.com/docs/api/charges
// that matters to the majority of client code.
type stripeCharge struct {
	ID                 string              `json:"id"`
	Object             string              `json:"object"`
	Amount             int64               `json:"amount"`
	AmountRefunded     int64               `json:"amount_refunded"`
	Captured           bool                `json:"captured"`
	Created            int64               `json:"created"`
	Currency           string              `json:"currency"`
	Customer           string              `json:"customer"`
	Description        *string             `json:"description"`
	Livemode           bool                `json:"livemode"`
	Paid               bool                `json:"paid"`
	Refunded           bool                `json:"refunded"`
	Status             string              `json:"status"`
	FailureCode        *string             `json:"failure_code"`
	FailureMessage     *string             `json:"failure_message"`
	Outcome            stripeChargeOutcome `json:"outcome"`
	PaymentMethodTypes []string            `json:"payment_method_types"`
}

type stripeChargeOutcome struct {
	NetworkStatus string  `json:"network_status"`
	Reason        *string `json:"reason"`
	RiskLevel     string  `json:"risk_level"`
	SellerMessage string  `json:"seller_message"`
	Type          string  `json:"type"`
}

func (s *Stripe) FormatCharge(r engine.ChargeResult) ([]byte, error) {
	ch := r.Charge
	out := stripeCharge{
		ID:                 ch.ID,
		Object:             "charge",
		Amount:             ch.Amount,
		AmountRefunded:     ch.Refunded,
		Captured:           !r.Outcome.IsFailure(),
		Created:            ch.Created,
		Currency:           ch.Currency,
		Customer:           ch.Customer,
		Livemode:           false,
		Paid:               !r.Outcome.IsFailure(),
		Refunded:           false,
		PaymentMethodTypes: []string{"card"},
	}
	if r.Outcome.IsFailure() {
		code := string(r.Outcome)
		msg := r.Outcome.Message()
		out.Status = "failed"
		out.FailureCode = &code
		out.FailureMessage = &msg
		out.Outcome = stripeChargeOutcome{
			NetworkStatus: "declined_by_network",
			Reason:        &code,
			RiskLevel:     "normal",
			SellerMessage: msg,
			Type:          "issuer_declined",
		}
	} else {
		out.Status = "succeeded"
		out.Outcome = stripeChargeOutcome{
			NetworkStatus: "approved_by_network",
			RiskLevel:     "normal",
			SellerMessage: "Payment complete.",
			Type:          "authorized",
		}
	}
	return marshal(out, s.Pretty)
}

type stripeRefund struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Amount   int64  `json:"amount"`
	Charge   string `json:"charge"`
	Created  int64  `json:"created"`
	Status   string `json:"status"`
	Reason   string `json:"reason"`
}

func (s *Stripe) FormatRefund(r engine.RefundResult) ([]byte, error) {
	return marshal(stripeRefund{
		ID:      r.Refund.ID,
		Object:  "refund",
		Amount:  r.Refund.Amount,
		Charge:  r.Refund.Charge,
		Created: r.Refund.Created,
		Status:  r.Refund.Status,
		Reason:  "requested_by_customer",
	}, s.Pretty)
}

type stripeCustomer struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
}

func (s *Stripe) FormatCustomer(c store.Customer) ([]byte, error) {
	return marshal(stripeCustomer{
		ID: c.ID, Object: "customer", Email: c.Email, Name: c.Name, Created: c.Created,
	}, s.Pretty)
}

type stripeList struct {
	Object  string           `json:"object"`
	Data    []stripeCustomer `json:"data"`
	HasMore bool             `json:"has_more"`
	URL     string           `json:"url"`
}

func (s *Stripe) FormatCustomers(cs []store.Customer) ([]byte, error) {
	list := stripeList{Object: "list", URL: "/v1/customers", Data: make([]stripeCustomer, 0, len(cs))}
	for _, c := range cs {
		list.Data = append(list.Data, stripeCustomer{
			ID: c.ID, Object: "customer", Email: c.Email, Name: c.Name, Created: c.Created,
		})
	}
	return marshal(list, s.Pretty)
}

type stripeSub struct {
	ID       string `json:"id"`
	Object   string `json:"object"`
	Customer string `json:"customer"`
	Status   string `json:"status"`
	Plan     struct {
		ID     string `json:"id"`
		Object string `json:"object"`
	} `json:"plan"`
	Created int64 `json:"created"`
}

func (s *Stripe) FormatSubscription(r engine.SubscribeResult) ([]byte, error) {
	out := stripeSub{
		ID:       r.Subscription.ID,
		Object:   "subscription",
		Customer: r.Subscription.Customer,
		Status:   r.Subscription.Status,
		Created:  r.Subscription.Created,
	}
	out.Plan.ID = r.Subscription.Plan
	out.Plan.Object = "plan"
	return marshal(out, s.Pretty)
}

// stripeError mirrors Stripe's top-level error envelope.
type stripeError struct {
	Error stripeErrorBody `json:"error"`
}

type stripeErrorBody struct {
	Type        string `json:"type"`
	Code        string `json:"code"`
	DeclineCode string `json:"decline_code,omitempty"`
	Message     string `json:"message"`
	Param       string `json:"param,omitempty"`
}

func (s *Stripe) FormatError(o engine.Outcome) ([]byte, error) {
	body := stripeErrorBody{Message: o.Message()}
	switch o {
	case engine.OutcomeCardDeclined:
		body.Type = "card_error"
		body.Code = "card_declined"
	case engine.OutcomeInsufficient:
		body.Type = "card_error"
		body.Code = "card_declined"
		body.DeclineCode = "insufficient_funds"
	case engine.OutcomeExpiredCard:
		body.Type = "card_error"
		body.Code = "expired_card"
	case engine.OutcomeProcessingError:
		body.Type = "api_error"
		body.Code = "processing_error"
	case engine.OutcomeRateLimited:
		body.Type = "rate_limit_error"
		body.Code = "rate_limit"
	default:
		body.Type = "api_error"
		body.Code = string(o)
	}
	return marshal(stripeError{Error: body}, s.Pretty)
}
