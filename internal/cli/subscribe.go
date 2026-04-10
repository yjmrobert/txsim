package cli

import (
	"github.com/spf13/cobra"

	"github.com/yjmrobert/txsim/internal/engine"
)

func newSubscribeCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	var (
		plan     string
		customer string
	)

	cmd := &cobra.Command{
		Use:   "subscribe",
		Short: "Create a mock subscription for a customer",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}

			outcome, err := engine.ParseOutcome(app.Flags.Status)
			if err != nil {
				return &ExitError{Code: ExitUsage, Err: err}
			}

			result, err := app.Engine.Subscribe(engine.SubscribeParams{
				Plan:     plan,
				Customer: customer,
				Outcome:  outcome,
				Delay:    app.Flags.Delay,
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

			body, ferr := app.Formatter.FormatSubscription(result)
			if ferr != nil {
				return &ExitError{Code: ExitState, Err: ferr}
			}
			return writeJSON(app.Out, body)
		},
	}

	cmd.Flags().StringVar(&plan, "plan", "", "plan identifier — required")
	cmd.Flags().StringVar(&customer, "customer", "", "customer ID — required")
	_ = cmd.MarkFlagRequired("plan")
	_ = cmd.MarkFlagRequired("customer")

	return cmd
}
