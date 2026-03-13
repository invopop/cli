package cmd

import (
	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func siloStatesCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	states := &cobra.Command{
		Use:   "states",
		Short: "Manage silo entry states",
	}

	states.AddCommand(siloStatesListCmd(getClient))
	states.AddCommand(siloStatesCreateCmd(getClient))

	return states
}

func siloStatesListCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "list <entry-id>",
		Short: "List states for a silo entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			col, err := c.Silo().States().List(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(col)
		},
	}
}

func siloStatesCreateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		key   string
		notes string
	)
	cmd := &cobra.Command{
		Use:   "create <entry-id>",
		Short: "Create a state for a silo entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			s, err := c.Silo().States().Create(cmd.Context(), &invopop.CreateSiloState{
				EntryID: args[0],
				Key:     key,
				Notes:   notes,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(s)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "State key (required)")
	cmd.Flags().StringVar(&notes, "notes", "", "Notes about the state transition")
	return cmd
}
