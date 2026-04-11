// Package cli wires the MockPay CLI commands to the engine and provider
// packages. Exit semantics are encoded via ExitError so main() can translate
// them into process exit codes.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/yjmrobert/txsim/internal/engine"
	"github.com/yjmrobert/txsim/internal/provider"
	"github.com/yjmrobert/txsim/internal/store"
)

// Exit code constants. These mirror the plan in
// /root/.claude/plans/async-zooming-phoenix.md.
const (
	ExitOK            = 0
	ExitPaymentFailed = 1
	ExitUsage         = 2
	ExitState         = 3
)

// ExitError wraps an error with a desired process exit code.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// globalFlags holds flag values shared across all subcommands.
type globalFlags struct {
	Mode      string
	Status    string
	Delay     int
	StateFile string
	Pretty    bool
}

// appContext bundles everything a subcommand needs to execute.
type appContext struct {
	Flags     globalFlags
	Engine    *engine.Engine
	Formatter provider.Formatter
	Out       io.Writer
}

// NewRootCmd builds the root `mockpay` command. out and errOut are the writers
// responses and errors are written to; tests inject buffers.
func NewRootCmd(out, errOut io.Writer) *cobra.Command {
	flags := &globalFlags{}

	root := &cobra.Command{
		Use:   "mockpay",
		Short: "MockPay - a local, provider-agnostic payment simulator for agents and CI",
		Long: `MockPay simulates complex payment workflows without live API keys.

It is designed for AI agents and automated tests that need deterministic,
offline responses shaped like a real payment provider (Stripe today; PayPal
and crypto layers planned).`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&flags.Mode, "mode", "generic", "provider mode: generic | stripe")
	root.PersistentFlags().StringVar(&flags.Status, "status", "", "force a deterministic outcome (e.g. card_declined, insufficient_funds)")
	root.PersistentFlags().IntVar(&flags.Delay, "delay", 0, "simulate latency in milliseconds before completing the operation")
	root.PersistentFlags().StringVar(&flags.StateFile, "state-file", "", "override the mock state file (default ~/.mockpay/state.json)")
	root.PersistentFlags().BoolVar(&flags.Pretty, "pretty", false, "pretty-print JSON output (default: compact)")

	// ctxFor builds a fresh appContext for each command invocation. Doing it
	// lazily (as opposed to in PersistentPreRun) means unit tests can call
	// NewRootCmd without a state file existing.
	ctxFor := func(cmd *cobra.Command) (*appContext, error) {
		f, err := provider.Get(flags.Mode, flags.Pretty)
		if err != nil {
			return nil, &ExitError{Code: ExitUsage, Err: err}
		}
		st, err := store.Open(flags.StateFile)
		if err != nil {
			return nil, &ExitError{Code: ExitState, Err: err}
		}
		w := out
		if w == nil {
			w = cmd.OutOrStdout()
		}
		return &appContext{
			Flags:     *flags,
			Engine:    engine.New(st),
			Formatter: f,
			Out:       w,
		}, nil
	}

	root.AddCommand(
		newChargeCmd(ctxFor),
		newRefundCmd(ctxFor),
		newCustomerCmd(ctxFor),
		newSubscribeCmd(ctxFor),
		newResetCmd(ctxFor),
	)

	if errOut != nil {
		root.SetErr(errOut)
	}
	if out != nil {
		root.SetOut(out)
	}

	return root
}

// Execute is the process entrypoint. It runs the root command and returns the
// intended exit code.
func Execute() int {
	root := NewRootCmd(os.Stdout, os.Stderr)
	err := root.Execute()
	if err == nil {
		return ExitOK
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		if exitErr.Code != ExitPaymentFailed {
			// Payment failures write provider-shaped JSON to stdout already;
			// for everything else, surface the error on stderr.
			fmt.Fprintln(os.Stderr, "error:", exitErr.Err)
		}
		return exitErr.Code
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	return ExitUsage
}

// writeJSON writes b followed by a newline. Centralising this makes it easier
// to keep stdout output consistent.
func writeJSON(w io.Writer, b []byte) error {
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}

// mapEngineError converts engine errors into ExitError values with the right
// exit code.
func mapEngineError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, engine.ErrInvalidInput) || errors.Is(err, engine.ErrNotFound) {
		return &ExitError{Code: ExitUsage, Err: err}
	}
	return &ExitError{Code: ExitState, Err: err}
}
