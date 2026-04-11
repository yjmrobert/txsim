// Package engine implements the core MockPay state transitions: creating
// customers, charging them, refunding charges, and subscribing them to plans.
// The engine is intentionally decoupled from the CLI and from any specific
// provider formatter - it takes a *store.Store and returns plain structs.
package engine

import (
	"errors"
	"fmt"
	"time"

	"github.com/yjmrobert/txsim/internal/store"
)

// ErrNotFound is returned when a referenced resource does not exist.
var ErrNotFound = errors.New("not found")

// ErrInvalidInput is returned for bad user-supplied values.
var ErrInvalidInput = errors.New("invalid input")

// Engine executes mock payment operations against a Store.
type Engine struct {
	Store *store.Store
	Now   func() time.Time
}

// New returns an Engine bound to s. Callers may override Now for tests.
func New(s *store.Store) *Engine {
	return &Engine{Store: s, Now: time.Now}
}

// CreateCustomerParams holds arguments for CreateCustomer.
type CreateCustomerParams struct {
	Email string
	Name  string
	Delay int
}

// CreateCustomer persists a new customer and returns it.
func (e *Engine) CreateCustomer(p CreateCustomerParams) (store.Customer, error) {
	if p.Email == "" {
		return store.Customer{}, fmt.Errorf("%w: --email is required", ErrInvalidInput)
	}
	Delay(p.Delay)
	c := store.Customer{
		ID:      NewID("cus"),
		Email:   p.Email,
		Name:    p.Name,
		Created: e.Now().Unix(),
	}
	err := e.Store.WithLock(func(st *store.State) error {
		st.Customers[c.ID] = c
		return nil
	})
	if err != nil {
		return store.Customer{}, err
	}
	return c, nil
}

// GetCustomer looks up a customer by ID.
func (e *Engine) GetCustomer(id string) (store.Customer, error) {
	var out store.Customer
	err := e.Store.WithLock(func(st *store.State) error {
		c, ok := st.Customers[id]
		if !ok {
			return fmt.Errorf("%w: customer %s", ErrNotFound, id)
		}
		out = c
		return nil
	})
	return out, err
}

// ListCustomers returns all persisted customers.
func (e *Engine) ListCustomers() ([]store.Customer, error) {
	var out []store.Customer
	err := e.Store.WithLock(func(st *store.State) error {
		for _, c := range st.Customers {
			out = append(out, c)
		}
		return nil
	})
	return out, err
}

// ChargeParams holds arguments for Charge.
type ChargeParams struct {
	Amount   int64
	Currency string
	Customer string
	Outcome  Outcome
	Delay    int
}

// ChargeResult is the result of a charge attempt.
type ChargeResult struct {
	Charge  store.Charge
	Outcome Outcome
}

// Charge attempts to charge a customer. If the Outcome is a failure, the
// charge is still persisted (with status "failed") so downstream queries
// reflect the attempt, but ChargeResult.Outcome carries the failure and the
// caller should surface a provider-shaped error.
func (e *Engine) Charge(p ChargeParams) (ChargeResult, error) {
	if p.Amount <= 0 {
		return ChargeResult{}, fmt.Errorf("%w: --amount must be > 0", ErrInvalidInput)
	}
	if p.Currency == "" {
		return ChargeResult{}, fmt.Errorf("%w: --currency is required", ErrInvalidInput)
	}
	if p.Customer == "" {
		return ChargeResult{}, fmt.Errorf("%w: --customer is required", ErrInvalidInput)
	}
	if p.Outcome == "" {
		p.Outcome = OutcomeSucceeded
	}
	Delay(p.Delay)

	var result ChargeResult
	err := e.Store.WithLock(func(st *store.State) error {
		if _, ok := st.Customers[p.Customer]; !ok {
			return fmt.Errorf("%w: customer %s", ErrNotFound, p.Customer)
		}
		ch := store.Charge{
			ID:       NewID("ch"),
			Amount:   p.Amount,
			Currency: p.Currency,
			Customer: p.Customer,
			Created:  e.Now().Unix(),
		}
		if p.Outcome.IsFailure() {
			ch.Status = "failed"
			ch.Outcome = string(p.Outcome)
			ch.FailureMsg = p.Outcome.Message()
		} else {
			ch.Status = "succeeded"
			ch.Outcome = string(OutcomeSucceeded)
		}
		st.Charges[ch.ID] = ch
		result = ChargeResult{Charge: ch, Outcome: p.Outcome}
		return nil
	})
	if err != nil {
		return ChargeResult{}, err
	}
	return result, nil
}

// RefundParams holds arguments for Refund.
type RefundParams struct {
	Charge  string
	Amount  int64 // 0 = full remaining amount
	Outcome Outcome
	Delay   int
}

// RefundResult is the result of a refund attempt.
type RefundResult struct {
	Refund  store.Refund
	Outcome Outcome
}

// Refund refunds (part of) a successful charge.
func (e *Engine) Refund(p RefundParams) (RefundResult, error) {
	if p.Charge == "" {
		return RefundResult{}, fmt.Errorf("%w: --charge is required", ErrInvalidInput)
	}
	if p.Outcome == "" {
		p.Outcome = OutcomeSucceeded
	}
	Delay(p.Delay)

	var result RefundResult
	err := e.Store.WithLock(func(st *store.State) error {
		ch, ok := st.Charges[p.Charge]
		if !ok {
			return fmt.Errorf("%w: charge %s", ErrNotFound, p.Charge)
		}
		if ch.Status != "succeeded" {
			return fmt.Errorf("%w: charge %s is not refundable (status=%s)", ErrInvalidInput, p.Charge, ch.Status)
		}
		remaining := ch.Amount - ch.Refunded
		amount := p.Amount
		if amount == 0 {
			amount = remaining
		}
		if amount <= 0 || amount > remaining {
			return fmt.Errorf("%w: refund amount %d exceeds remaining %d", ErrInvalidInput, amount, remaining)
		}
		r := store.Refund{
			ID:      NewID("re"),
			Charge:  ch.ID,
			Amount:  amount,
			Created: e.Now().Unix(),
		}
		if p.Outcome.IsFailure() {
			r.Status = "failed"
		} else {
			r.Status = "succeeded"
			ch.Refunded += amount
			st.Charges[ch.ID] = ch
		}
		st.Refunds[r.ID] = r
		result = RefundResult{Refund: r, Outcome: p.Outcome}
		return nil
	})
	if err != nil {
		return RefundResult{}, err
	}
	return result, nil
}

// SubscribeParams holds arguments for Subscribe.
type SubscribeParams struct {
	Plan     string
	Customer string
	Outcome  Outcome
	Delay    int
}

// SubscribeResult is the result of a subscribe attempt.
type SubscribeResult struct {
	Subscription store.Subscription
	Outcome      Outcome
}

// Subscribe creates a subscription for a customer.
func (e *Engine) Subscribe(p SubscribeParams) (SubscribeResult, error) {
	if p.Plan == "" {
		return SubscribeResult{}, fmt.Errorf("%w: --plan is required", ErrInvalidInput)
	}
	if p.Customer == "" {
		return SubscribeResult{}, fmt.Errorf("%w: --customer is required", ErrInvalidInput)
	}
	if p.Outcome == "" {
		p.Outcome = OutcomeSucceeded
	}
	Delay(p.Delay)

	var result SubscribeResult
	err := e.Store.WithLock(func(st *store.State) error {
		if _, ok := st.Customers[p.Customer]; !ok {
			return fmt.Errorf("%w: customer %s", ErrNotFound, p.Customer)
		}
		sub := store.Subscription{
			ID:       NewID("sub"),
			Plan:     p.Plan,
			Customer: p.Customer,
			Created:  e.Now().Unix(),
		}
		if p.Outcome.IsFailure() {
			sub.Status = "past_due"
		} else {
			sub.Status = "active"
		}
		st.Subscriptions[sub.ID] = sub
		result = SubscribeResult{Subscription: sub, Outcome: p.Outcome}
		return nil
	})
	if err != nil {
		return SubscribeResult{}, err
	}
	return result, nil
}

// Reset wipes all persisted state.
func (e *Engine) Reset() error {
	return e.Store.Reset()
}
