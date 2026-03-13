package cmd

import (
	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func sequenceEntriesCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	entries := &cobra.Command{
		Use:   "entries",
		Short: "Manage sequence entries",
	}

	entries.AddCommand(sequenceEntriesCreateCmd(getClient))
	entries.AddCommand(sequenceEntriesFetchCmd(getClient))

	return entries
}

func sequenceEntriesCreateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "create <series-id>",
		Short: "Create an entry in a series",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			entry, err := c.Sequence().CreateEntry(cmd.Context(), args[0], &invopop.CreateSeriesEntry{})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(entry)
		},
	}
}

func sequenceEntriesFetchCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <series-id> <id>",
		Short: "Fetch an entry from a series",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			entry, err := c.Sequence().FetchEntry(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(entry)
		},
	}
}

func sequenceCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	seq := &cobra.Command{
		Use:   "sequence",
		Short: "Manage sequences and series",
	}

	seq.AddCommand(sequenceSeriesCmd(getClient))
	seq.AddCommand(sequenceEntriesCmd(getClient))

	return seq
}
