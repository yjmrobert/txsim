package cli

import (
	"github.com/spf13/cobra"

	"github.com/yjmrobert/txsim/internal/engine"
)

func newRefundCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	var (
		charge string
		amount int64
	)

	cmd := &cobra.Command{
		Use:   "refund",
		Short: "Refund all or part of a mock charge",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}

			outcome, err := engine.ParseOutcome(app.Flags.Status)
			if err != nil {
				return &ExitError{Code: ExitUsage, Err: err}
			}

			result, err := app.Engine.Refund(engine.RefundParams{
				Charge:  charge,
				Amount:  amount,
				Outcome: outcome,
				Delay:   app.Flags.Delay,
			})
			if err != nil {
				return mapEngineError(err)
			}

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

			body, ferr := app.Formatter.FormatRefund(result)
			if ferr != nil {
				return &ExitError{Code: ExitState, Err: ferr}
			}
			return writeJSON(app.Out, body)
		},
	}

	cmd.Flags().StringVar(&charge, "charge", "", "charge ID to refund — required")
	cmd.Flags().Int64Var(&amount, "amount", 0, "partial refund amount (defaults to full remaining)")
	_ = cmd.MarkFlagRequired("charge")

	return cmd
}
