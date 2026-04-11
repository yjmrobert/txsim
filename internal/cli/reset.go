package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newResetCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Delete all mock state (customers, charges, refunds, subscriptions)",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}
			if err := app.Engine.Reset(); err != nil {
				return &ExitError{Code: ExitState, Err: err}
			}
			fmt.Fprintln(app.Out, `{"type":"reset","status":"ok"}`)
			return nil
		},
	}
}
