// Package store persists MockPay's mock state to a local JSON file.
package store

// StateVersion is the on-disk schema version. Bump when the layout changes.
const StateVersion = 1

// State is the root document persisted to ~/.mockpay/state.json.
type State struct {
	Version       int                     `json:"version"`
	Customers     map[string]Customer     `json:"customers"`
	Charges       map[string]Charge       `json:"charges"`
	Refunds       map[string]Refund       `json:"refunds"`
	Subscriptions map[string]Subscription `json:"subscriptions"`
}

// NewState returns an empty State ready to be populated.
func NewState() *State {
	return &State{
		Version:       StateVersion,
		Customers:     map[string]Customer{},
		Charges:       map[string]Charge{},
		Refunds:       map[string]Refund{},
		Subscriptions: map[string]Subscription{},
	}
}

// Customer is a stored customer record.
type Customer struct {
	ID      string `json:"id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Created int64  `json:"created"`
}

// Charge is a stored charge record. Amounts are in the smallest currency unit
// (e.g. cents) to match how real providers like Stripe represent money.
type Charge struct {
	ID         string `json:"id"`
	Amount     int64  `json:"amount"`
	Currency   string `json:"currency"`
	Customer   string `json:"customer"`
	Status     string `json:"status"`      // succeeded | failed
	Outcome    string `json:"outcome"`     // outcome code, e.g. "card_declined"
	FailureMsg string `json:"failure_msg,omitempty"`
	Refunded   int64  `json:"refunded"`    // cumulative refunded amount
	Created    int64  `json:"created"`
}

// Refund is a stored refund record.
type Refund struct {
	ID      string `json:"id"`
	Charge  string `json:"charge"`
	Amount  int64  `json:"amount"`
	Status  string `json:"status"`
	Created int64  `json:"created"`
}

// Subscription is a stored subscription record.
type Subscription struct {
	ID       string `json:"id"`
	Plan     string `json:"plan"`
	Customer string `json:"customer"`
	Status   string `json:"status"` // active | past_due | canceled
	Created  int64  `json:"created"`
}
