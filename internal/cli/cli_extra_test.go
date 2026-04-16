package cli_test

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

	"github.com/yjmrobert/txsim/internal/cli"
	"github.com/yjmrobert/txsim/internal/engine"
)

// runCmd runs the root command with the given args against stateFile and
// returns (stdout, stderr, err). Tests use this instead of touching the real
// ~/.mockpay directory.
func runCmd(t *testing.T, stateFile string, args ...string) (string, string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root := cli.NewRootCmd(&stdout, &stderr)
	full := append([]string{"--state-file", stateFile}, args...)
	root.SetArgs(full)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

func TestSubscribeHappyPath(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")

	out, _, err := runCmd(t, sf, "customer", "create", "--email", "a@b.com", "--name", "Alice")
	require.NoError(t, err)
	cus := mustID(t, out)

	out, _, err = runCmd(t, sf, "--mode", "stripe", "subscribe", "--plan", "gold", "--customer", cus)
	require.NoError(t, err)

	var sub map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &sub))
	assert.Equal(t, "subscription", sub["object"])
	assert.Equal(t, "active", sub["status"])
	assert.True(t, strings.HasPrefix(sub["id"].(string), "sub_"))
}

func TestSubscribeFailureStatusExits1(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, _ := runCmd(t, sf, "customer", "create", "--email", "a@b.com")
	cus := mustID(t, out)

	out, _, err := runCmd(t, sf,
		"--mode", "stripe", "--status", "insufficient_funds",
		"subscribe", "--plan", "gold", "--customer", cus)
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitPaymentFailed, ex.Code)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	envelope := decoded["error"].(map[string]any)
	assert.Equal(t, "insufficient_funds", envelope["decline_code"])
}

func TestCustomerGetHappyPath(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")

	out, _, err := runCmd(t, sf, "customer", "create", "--email", "a@b.com", "--name", "Alice")
	require.NoError(t, err)
	cus := mustID(t, out)

	out, _, err = runCmd(t, sf, "customer", "get", cus)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, cus, decoded["id"])
}

func TestCustomerGetRequiresExactlyOneArg(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "customer", "get")
	require.Error(t, err)
	_, _, err = runCmd(t, sf, "customer", "get", "a", "b")
	require.Error(t, err)
}

func TestRefundFailureStatusExits1(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")

	out, _, _ := runCmd(t, sf, "customer", "create", "--email", "a@b.com")
	cus := mustID(t, out)
	out, _, _ = runCmd(t, sf, "charge", "--amount", "1000", "--currency", "usd", "--customer", cus)
	ch := mustID(t, out)

	out, _, err := runCmd(t, sf, "--status", "processing_error", "refund", "--charge", ch)
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitPaymentFailed, ex.Code)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "error", decoded["type"])
}

func TestRefundMissingChargeIsUsageError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "refund", "--charge", "ch_missing")
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}

func TestPrettyFlagIndentsOutput(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")

	out, _, err := runCmd(t, sf, "--pretty", "customer", "create", "--email", "a@b.com", "--name", "Alice")
	require.NoError(t, err)
	assert.Contains(t, out, "\n  ", "--pretty should produce indented JSON")
}

func TestChargeEqualsFormFlag(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, _ := runCmd(t, sf, "customer", "create", "--email", "a@b.com")
	cus := mustID(t, out)

	// --mode=stripe (equals form) should work identically to --mode stripe.
	out, _, err := runCmd(t, sf, "--mode=stripe", "charge",
		"--amount=1000", "--currency=usd", "--customer="+cus)
	require.NoError(t, err)
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	assert.Equal(t, "charge", decoded["object"])
}

func TestInvalidStatusFlagIsUsageError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "--status", "nope",
		"charge", "--amount", "1", "--currency", "usd", "--customer", "cus_x")
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}

func TestInvalidAmountIsUsageError(t *testing.T) {
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, _ := runCmd(t, sf, "customer", "create", "--email", "a@b.com")
	cus := mustID(t, out)

	_, _, err := runCmd(t, sf, "charge", "--amount", "0", "--currency", "usd", "--customer", cus)
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Equal(t, cli.ExitUsage, ex.Code)
}

func TestHelpDoesNotError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	// Root help
	out, _, err := runCmd(t, sf, "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "mockpay")
	assert.Contains(t, out, "charge")
	assert.Contains(t, out, "customer")

	// Subcommand help
	out, _, err = runCmd(t, sf, "charge", "--help")
	require.NoError(t, err)
	assert.Contains(t, out, "--amount")
	assert.Contains(t, out, "--customer")
}

func TestMissingRequiredFlagIsError(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, stderr, err := runCmd(t, sf, "charge", "--currency", "usd", "--customer", "cus_x")
	require.Error(t, err, "missing --amount should fail")
	assert.Contains(t, strings.ToLower(stderr+err.Error()), "amount")
}

func TestResetIsIdempotent(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, err := runCmd(t, sf, "reset")
	require.NoError(t, err)
	_, _, err = runCmd(t, sf, "reset")
	require.NoError(t, err, "reset on already-empty state should still succeed")
}

func TestChargeFailedErrorMessageIsDescriptive(t *testing.T) {
	// The chargeFailed sentinel embedded in ExitError should carry enough
	// information to be diagnostic if it ever reaches stderr.
	engine.Sleeper = func(time.Duration) {}
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, _ := runCmd(t, sf, "customer", "create", "--email", "a@b.com")
	cus := mustID(t, out)

	_, _, err := runCmd(t, sf, "--status", "card_declined",
		"charge", "--amount", "100", "--currency", "usd", "--customer", cus)
	require.Error(t, err)
	var ex *cli.ExitError
	require.True(t, errors.As(err, &ex))
	assert.Contains(t, ex.Error(), "card_declined")
}

func mustID(t *testing.T, jsonOut string) string {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(jsonOut), &decoded))
	id, ok := decoded["id"].(string)
	require.True(t, ok, "expected string id field")
	return id
}
