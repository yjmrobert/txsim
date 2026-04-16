package engine_test

import (
	"fmt"
	"path/filepath"
	"os"
	"time"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/store"
)

// ExampleEngine_Charge shows the canonical "create customer then charge
// them" flow. Runs as a test (so it stays correct) and appears on pkg.go.dev.
func ExampleEngine_Charge() {
	dir, _ := os.MkdirTemp("", "mockpay-doc-*")
	defer os.RemoveAll(dir)

	s, _ := store.Open(filepath.Join(dir, "state.json"))
	e := engine.New(s)
	// Pin the clock so the example output is stable.
	e.Now = func() time.Time { return time.Unix(1700000000, 0) }
	engine.Sleeper = func(time.Duration) {}

	c, _ := e.CreateCustomer(engine.CreateCustomerParams{Email: "alice@example.com", Name: "Alice"})
	r, _ := e.Charge(engine.ChargeParams{
		Amount: 2500, Currency: "usd", Customer: c.ID,
	})
	fmt.Println(r.Charge.Status, r.Charge.Amount, r.Charge.Currency)
	// Output: succeeded 2500 usd
}
