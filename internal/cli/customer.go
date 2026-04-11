package cli

import (
	"sort"

	"github.com/spf13/cobra"

	"github.com/yjmrobert/txsim/internal/engine"
)

func newCustomerCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "customer",
		Short: "Manage mock customers",
	}
	cmd.AddCommand(newCustomerCreateCmd(ctxFor))
	cmd.AddCommand(newCustomerGetCmd(ctxFor))
	cmd.AddCommand(newCustomerListCmd(ctxFor))
	return cmd
}

func newCustomerCreateCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	var (
		email string
		name  string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new mock customer",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}
			c, err := app.Engine.CreateCustomer(engine.CreateCustomerParams{
				Email: email,
				Name:  name,
				Delay: app.Flags.Delay,
			})
			if err != nil {
				return mapEngineError(err)
			}
			body, ferr := app.Formatter.FormatCustomer(c)
			if ferr != nil {
				return &ExitError{Code: ExitState, Err: ferr}
			}
			return writeJSON(app.Out, body)
		},
	}
	cmd.Flags().StringVar(&email, "email", "", "customer email — required")
	cmd.Flags().StringVar(&name, "name", "", "customer display name")
	_ = cmd.MarkFlagRequired("email")
	return cmd
}

func newCustomerGetCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "get <customer-id>",
		Short: "Fetch a mock customer by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}
			c, err := app.Engine.GetCustomer(args[0])
			if err != nil {
				return mapEngineError(err)
			}
			body, ferr := app.Formatter.FormatCustomer(c)
			if ferr != nil {
				return &ExitError{Code: ExitState, Err: ferr}
			}
			return writeJSON(app.Out, body)
		},
	}
}

func newCustomerListCmd(ctxFor func(*cobra.Command) (*appContext, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all mock customers",
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := ctxFor(cmd)
			if err != nil {
				return err
			}
			cs, err := app.Engine.ListCustomers()
			if err != nil {
				return mapEngineError(err)
			}
			// Stable ordering makes output scriptable.
			sort.Slice(cs, func(i, j int) bool { return cs[i].Created < cs[j].Created })
			body, ferr := app.Formatter.FormatCustomers(cs)
			if ferr != nil {
				return &ExitError{Code: ExitState, Err: ferr}
			}
			return writeJSON(app.Out, body)
		},
	}
}

