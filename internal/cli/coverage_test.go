package cli_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/cli"
	"github.com/yjmrobert/txsim/internal/engine"
)

// TestCustomerCreateMissingEmailIsUsageError exercises the engine's
// ErrInvalidInput branch inside newCustomerCreateCmd.
func TestCustomerCreateMissingEmailIsUsageError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	// --email is marked required by cobra, so omitting it produces a cobra
	// error, not an engine error. Providing an empty value bypasses cobra's
	// check and reaches engine.ErrInvalidInput.
	_, _, err := runCmd(t, sf, "customer", "create", "--email", "")
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}

// TestSubscribeMissingPlanIsUsageError covers the engine-error path inside
// newSubscribeCmd after cobra's required-flag check is satisfied.
func TestSubscribeMissingCustomerIsUsageError(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "subscribe", "--plan", "gold", "--customer", "cus_missing")
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}

// TestResetThenListWorksEndToEnd ensures the reset -> list happy path
// contributes coverage for newResetCmd's success branch.
func TestResetThenListWorksEndToEnd(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")

	_, _, err := runCmd(t, sf, "customer", "create", "--email", "a@b.com")
	require.NoError(t, err)

	out, _, err := runCmd(t, sf, "reset")
	require.NoError(t, err)
	assert.Contains(t, out, `"status":"ok"`)

	out, _, err = runCmd(t, sf, "customer", "list")
	require.NoError(t, err)
	assert.Contains(t, out, `"data":[]`)
}

// TestSubscribeInvalidStatusIsUsageError covers the ParseOutcome error branch
// inside newSubscribeCmd.
func TestSubscribeInvalidStatusIsUsageError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "--status", "nope", "subscribe", "--plan", "gold", "--customer", "cus_x")
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}

// TestRefundInvalidStatusIsUsageError covers the ParseOutcome error branch
// inside newRefundCmd.
func TestRefundInvalidStatusIsUsageError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "--status", "nope", "refund", "--charge", "ch_x")
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}
