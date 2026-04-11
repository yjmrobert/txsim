package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yjmrobert/txsim/internal/engine"
)

// runCmd invokes the root command with the provided args against a throwaway
// state file so tests don't touch the real ~/.mockpay directory.
func runCmd(t *testing.T, stateFile string, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := NewRootCmd(&stdout, &stderr)
	// Inject the temp state file as the first flag so it wins regardless of
	// subcommand placement.
	full := append([]string{"--state-file", stateFile}, args...)
	root.SetArgs(full)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestEndToEndStripeFlow(t *testing.T) {
	// Make sure no test accidentally inherits real sleep behaviour.
	engine.Sleeper = func(time.Duration) {}
	stateFile := filepath.Join(t.TempDir(), "state.json")

	// 1. Create customer in Stripe mode.
	out, _, err := runCmd(t, stateFile, "--mode", "stripe", "customer", "create", "--email", "a@b.com", "--name", "Alice")
	require.NoError(t, err)

	var customer map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &customer))
	assert.Equal(t, "customer", customer["object"])
	cusID, _ := customer["id"].(string)
	require.True(t, strings.HasPrefix(cusID, "cus_"))

	// 2. Happy-path charge.
	out, _, err = runCmd(t, stateFile, "--mode", "stripe", "charge", "--amount", "2500", "--currency", "usd", "--customer", cusID)
	require.NoError(t, err)
	var charge map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &charge))
	assert.Equal(t, "charge", charge["object"])
	assert.Equal(t, "succeeded", charge["status"])
	chID, _ := charge["id"].(string)
	require.True(t, strings.HasPrefix(chID, "ch_"))

	// 3. Deterministic failure should emit a Stripe-shaped error envelope
	// AND return the "payment failed" exit code.
	out, _, err = runCmd(t, stateFile, "--mode", "stripe", "--status", "card_declined",
		"charge", "--amount", "500", "--currency", "usd", "--customer", cusID)
	require.Error(t, err)
	var ex *ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, ExitPaymentFailed, ex.Code)

	var errEnvelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &errEnvelope))
	body := errEnvelope["error"].(map[string]any)
	assert.Equal(t, "card_error", body["type"])
	assert.Equal(t, "card_declined", body["code"])

	// 4. Refund the successful charge.
	out, _, err = runCmd(t, stateFile, "--mode", "stripe", "refund", "--charge", chID)
	require.NoError(t, err)
	var refund map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &refund))
	assert.Equal(t, "refund", refund["object"])
	assert.EqualValues(t, 2500, refund["amount"])

	// 5. customer list should include the customer we created earlier.
	out, _, err = runCmd(t, stateFile, "--mode", "stripe", "customer", "list")
	require.NoError(t, err)
	var list map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	assert.Equal(t, "list", list["object"])
	data, _ := list["data"].([]any)
	assert.Len(t, data, 1)

	// 6. Reset wipes state.
	_, _, err = runCmd(t, stateFile, "reset")
	require.NoError(t, err)

	out, _, err = runCmd(t, stateFile, "customer", "list")
	require.NoError(t, err)
	var genericList map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &genericList))
	emptyData, _ := genericList["data"].([]any)
	assert.Empty(t, emptyData)
}

func TestUnknownModeIsUsageError(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, stateFile, "--mode", "paypal", "customer", "list")
	require.Error(t, err)
	var ex *ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, ExitUsage, ex.Code)
}

func TestUnknownCustomerIsUsageError(t *testing.T) {
	stateFile := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, stateFile, "charge", "--amount", "100", "--currency", "usd", "--customer", "cus_missing")
	require.Error(t, err)
	var ex *ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, ExitUsage, ex.Code)
}
