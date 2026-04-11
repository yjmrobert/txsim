package cli

import (
	"github.com/spf13/cobra"

	"github.com/yjmrobert/txsim/internal/engine"
)

func newChargeCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	var (
		amount   int64
		currency string
		customer string
	)

	cmd := &cobra.Command{
		Use:   "charge",
		Short: "Create a mock charge against a customer",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}

			outcome, err := engine.ParseOutcome(app.Flags.Status)
			if err != nil {
				return &ExitError{Code: ExitUsage, Err: err}
			}

			result, err := app.Engine.Charge(engine.ChargeParams{
				Amount:   amount,
				Currency: currency,
				Customer: customer,
				Outcome:  outcome,
				Delay:    app.Flags.Delay,
			})
			if err != nil {
				return mapEngineError(err)
			}

			// Failed payments emit a provider-shaped error envelope AND a
			// non-zero exit code, so agents can branch on either signal.
			if result.Outcome.IsFailure() {
				body, ferr := app.Formatter.FormatError(result.Outcome)
				if ferr != nil {
					return &ExitError{Code: ExitState, Err: ferr}
				}
				if werr := writeJSON(app.Out, body); werr != nil {
					return &ExitError{Code: ExitState, Err: werr}
				}
				return &ExitError{Code: ExitPaymentFailed, Err: chargeFailed{outcome: result.Outcome}}
			}

			body, ferr := app.Formatter.FormatCharge(result)
			if ferr != nil {
				return &ExitError{Code: ExitState, Err: ferr}
			}
			return writeJSON(app.Out, body)
		},
	}

	cmd.Flags().Int64Var(&amount, "amount", 0, "amount in the smallest currency unit (e.g. cents) — required")
	cmd.Flags().StringVar(&currency, "currency", "", "currency code (e.g. usd) — required")
	cmd.Flags().StringVar(&customer, "customer", "", "customer ID — required")
	_ = cmd.MarkFlagRequired("amount")
	_ = cmd.MarkFlagRequired("currency")
	_ = cmd.MarkFlagRequired("customer")

	return cmd
}

// chargeFailed is used only to give ExitError a descriptive message for the
// unlikely case it leaks to stderr. Payment failures normally emit provider
// JSON on stdout and return exit code 1 silently.
type chargeFailed struct{ outcome engine.Outcome }

func (c chargeFailed) Error() string { return "charge failed: " + string(c.outcome) }
