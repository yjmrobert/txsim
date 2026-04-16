//go:build e2e

// Package e2e drives the built `mockpay` binary as a subprocess to exercise
// os.Exit semantics, real stdout/stderr separation, real flag parsing, and
// cross-process file locking — none of which the in-process cobra tests
// catch. Runs only when the `e2e` build tag is set:
//
//	go test -tags=e2e ./e2e/...
package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// binaryPath is resolved once in TestMain and reused across tests.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary into a temp dir. Doing this once per test run keeps
	// the suite snappy.
	tmp, err := os.MkdirTemp("", "mockpay-e2e-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "mkdir temp:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmp)

	binaryPath = filepath.Join(tmp, "mockpay")
	build := exec.Command("go", "build", "-o", binaryPath, "../cmd/mockpay")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "go build:", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// run invokes the built binary against stateFile and returns stdout, stderr,
// and the exit code. Non-zero exit codes are reported via the return value,
// not via a Go error — tests assert on the code directly.
func run(t *testing.T, stateFile string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	full := append([]string{"--state-file", stateFile}, args...)
	cmd := exec.Command(binaryPath, full...)
	var so, se bytes.Buffer
	cmd.Stdout = &so
	cmd.Stderr = &se
	err := cmd.Run()
	code := 0
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			t.Fatalf("exec failed: %v", err)
		}
	}
	return so.String(), se.String(), code
}

func idOf(t *testing.T, out string) string {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &decoded))
	id, _ := decoded["id"].(string)
	require.NotEmpty(t, id, "no id in output: %s", out)
	return id
}

func TestE2EHappyFlowStripeMode(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")

	// Create customer
	out, _, code := run(t, sf, "--mode", "stripe", "customer", "create", "--email", "a@b.com", "--name", "Alice")
	require.Equal(t, 0, code)
	cus := idOf(t, out)

	// Happy charge
	out, _, code = run(t, sf, "--mode", "stripe", "charge",
		"--amount", "2500", "--currency", "usd", "--customer", cus)
	require.Equal(t, 0, code)
	var charge map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &charge))
	assert.Equal(t, "charge", charge["object"])
	assert.Equal(t, "succeeded", charge["status"])

	// Refund
	_, _, code = run(t, sf, "--mode", "stripe", "refund", "--charge", charge["id"].(string))
	require.Equal(t, 0, code)

	// List should contain one customer.
	out, _, code = run(t, sf, "--mode", "stripe", "customer", "list")
	require.Equal(t, 0, code)
	var list map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	data, _ := list["data"].([]any)
	assert.Len(t, data, 1)
}

func TestE2EDeterministicFailureExitsCode1(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, _ := run(t, sf, "customer", "create", "--email", "a@b.com")
	cus := idOf(t, out)

	out, _, code := run(t, sf,
		"--mode", "stripe", "--status", "card_declined",
		"charge", "--amount", "500", "--currency", "usd", "--customer", cus)
	assert.Equal(t, 1, code, "payment failure should be exit 1")

	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &envelope))
	body := envelope["error"].(map[string]any)
	assert.Equal(t, "card_declined", body["code"])
}

func TestE2EUsageErrorExitsCode2(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, stderr, code := run(t, sf, "--mode", "paypal", "customer", "list")
	assert.Equal(t, 2, code)
	assert.Contains(t, strings.ToLower(stderr), "paypal")
}

func TestE2EUnknownCommandExitsNonZero(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, _, code := run(t, sf, "invent-a-command")
	assert.NotEqual(t, 0, code)
}

func TestE2EHelpExitsZero(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, code := run(t, sf, "--help")
	assert.Equal(t, 0, code)
	assert.Contains(t, out, "mockpay")
	assert.Contains(t, out, "charge")
}

func TestE2EMissingRequiredFlagExitsNonZero(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	_, stderr, code := run(t, sf, "charge", "--currency", "usd", "--customer", "cus_x")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, strings.ToLower(stderr), "amount")
}

// TestE2ECrossProcessConcurrency fires N subprocesses in parallel against the
// same state file and verifies all charges land. This is the actual
// protection provided by gofrs/flock — the in-process mutex cannot help here
// because each subprocess has its own Store instance.
func TestE2ECrossProcessConcurrency(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, _ := run(t, sf, "customer", "create", "--email", "a@b.com")
	cus := idOf(t, out)

	const n = 20
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			amount := fmt.Sprintf("%d", (i+1)*100)
			_, _, code := run(t, sf, "charge", "--amount", amount, "--currency", "usd", "--customer", cus)
			assert.Equal(t, 0, code)
		}()
	}
	wg.Wait()

	// Count persisted charges directly from disk.
	data, err := os.ReadFile(sf)
	require.NoError(t, err)
	var decoded struct {
		Charges map[string]any `json:"charges"`
	}
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Len(t, decoded.Charges, n, "all %d parallel charges should persist", n)
}

// TestE2EStateFileReadOnlyIsExitCode3 forces an IO error by stripping write
// permission from the state file and verifies the documented exit code.
func TestE2EStateFileReadOnlyIsExitCode3(t *testing.T) {
	dir := t.TempDir()
	sf := filepath.Join(dir, "state.json")

	// Seed the state file by creating a customer first.
	_, _, code := run(t, sf, "customer", "create", "--email", "a@b.com")
	require.Equal(t, 0, code)

	// Now make the *directory* read-only. On Linux this blocks the atomic
	// temp-file rename that Save() relies on, so the next mutation surfaces
	// ExitState (3).
	if os.Geteuid() == 0 {
		t.Skip("running as root bypasses file permissions — cannot exercise this path")
	}
	require.NoError(t, os.Chmod(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) }) // restore so TempDir can clean up

	_, _, code = run(t, sf, "customer", "create", "--email", "b@c.com")
	assert.Equal(t, 3, code, "IO errors should exit with code 3")
}

// TestE2EPrettyFlagIsIndented is a simple sanity check that the --pretty flag
// survives the real binary round-trip (argv parsing, JSON rendering).
func TestE2EPrettyFlagIsIndented(t *testing.T) {
	sf := filepath.Join(t.TempDir(), "state.json")
	out, _, code := run(t, sf, "--pretty", "customer", "create", "--email", "a@b.com", "--name", "Alice")
	require.Equal(t, 0, code)
	assert.Contains(t, out, "\n  ")
}
