package provider

import (
	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// Generic emits a flat, minimal schema optimized for LLM consumption. Every
// object carries a "type" discriminator so agents can dispatch on it.
type Generic struct{ Pretty bool }

func (g *Generic) Name() string { return "generic" }

type genericCharge struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Status   string `json:"status"`
	Outcome  string `json:"outcome"`
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
	Customer string `json:"customer"`
	Created  int64  `json:"created"`
	Message  string `json:"message,omitempty"`
}

func (g *Generic) FormatCharge(r engine.ChargeResult) ([]byte, error) {
	out := genericCharge{
		Type:     "charge",
		ID:       r.Charge.ID,
		Status:   r.Charge.Status,
		Outcome:  r.Charge.Outcome,
		Amount:   r.Charge.Amount,
		Currency: r.Charge.Currency,
		Customer: r.Charge.Customer,
		Created:  r.Charge.Created,
	}
	if r.Outcome.IsFailure() {
		out.Message = r.Outcome.Message()
	}
	return marshal(out, g.Pretty)
}

type genericRefund struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Charge  string `json:"charge"`
	Amount  int64  `json:"amount"`
	Status  string `json:"status"`
	Created int64  `json:"created"`
}

func (g *Generic) FormatRefund(r engine.RefundResult) ([]byte, error) {
	return marshal(genericRefund{
		Type:    "refund",
		ID:      r.Refund.ID,
		Charge:  r.Refund.Charge,
		Amount:  r.Refund.Amount,
		Status:  r.Refund.Status,
		Created: r.Refund.Created,
	}, g.Pretty)
}

type genericCustomer struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
}

func (g *Generic) FormatCustomer(c store.Customer) ([]byte, error) {
	return marshal(genericCustomer{
		Type: "customer", ID: c.ID, Email: c.Email, Name: c.Name, Created: c.Created,
	}, g.Pretty)
}

type genericCustomerList struct {
	Type string            `json:"type"`
	Data []genericCustomer `json:"data"`
}

func (g *Generic) FormatCustomers(cs []store.Customer) ([]byte, error) {
	list := genericCustomerList{Type: "list", Data: make([]genericCustomer, 0, len(cs))}
	for _, c := range cs {
		list.Data = append(list.Data, genericCustomer{
			Type: "customer", ID: c.ID, Email: c.Email, Name: c.Name, Created: c.Created,
		})
	}
	return marshal(list, g.Pretty)
}

type genericSub struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Plan     string `json:"plan"`
	Customer string `json:"customer"`
	Status   string `json:"status"`
	Created  int64  `json:"created"`
}

func (g *Generic) FormatSubscription(r engine.SubscribeResult) ([]byte, error) {
	return marshal(genericSub{
		Type:     "subscription",
		ID:       r.Subscription.ID,
		Plan:     r.Subscription.Plan,
		Customer: r.Subscription.Customer,
		Status:   r.Subscription.Status,
		Created:  r.Subscription.Created,
	}, g.Pretty)
}

type genericError struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (g *Generic) FormatError(o engine.Outcome) ([]byte, error) {
	return marshal(genericError{
		Type: "error", Code: string(o), Message: o.Message(),
	}, g.Pretty)
}
