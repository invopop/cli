package cmd

import (
	iohelper "github.com/invopop/cli/internal/io"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

func siloEntriesCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	entries := &cobra.Command{
		Use:   "entries",
		Short: "Manage silo entries",
	}

	entries.AddCommand(siloEntriesListCmd(getClient))
	entries.AddCommand(siloEntriesCreateCmd(getClient))
	entries.AddCommand(siloEntriesFetchCmd(getClient))
	entries.AddCommand(siloEntriesFetchKeyCmd(getClient))
	entries.AddCommand(siloEntriesUpdateCmd(getClient))
	entries.AddCommand(siloEntriesQueryCmd(getClient))
	entries.AddCommand(siloEntriesCountCmd(getClient))

	return entries
}

func siloEntriesListCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		folder string
		limit  int32
		cursor string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List silo entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			col, err := c.Silo().Entries().List(cmd.Context(), &invopop.FindSiloEntries{
				Folder: folder,
				Limit:  limit,
				Cursor: cursor,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(col)
		},
	}
	cmd.Flags().StringVar(&folder, "folder", "", "Filter by folder")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Maximum number of results")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	return cmd
}

func siloEntriesCreateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		file   string
		folder string
		key    string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a silo entry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := iohelper.ReadInput(file)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			entry, err := c.Silo().Entries().Create(cmd.Context(), &invopop.CreateSiloEntry{
				Data:   data,
				Folder: folder,
				Key:    key,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(entry)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to GOBL document (JSON or YAML)")
	cmd.Flags().StringVar(&folder, "folder", "", "Folder for the entry")
	cmd.Flags().StringVar(&key, "key", "", "Idempotency key")
	return cmd
}

func siloEntriesFetchCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <id>",
		Short: "Fetch a silo entry by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			entry, err := c.Silo().Entries().Fetch(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(entry)
		},
	}
}

func siloEntriesFetchKeyCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch-key <key>",
		Short: "Fetch a silo entry by key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			entry, err := c.Silo().Entries().FetchByKey(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(entry)
		},
	}
}

func siloEntriesUpdateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		file   string
		folder string
	)
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a silo entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := iohelper.ReadInput(file)
			if err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			entry, err := c.Silo().Entries().Update(cmd.Context(), &invopop.UpdateSiloEntry{
				ID:     args[0],
				Data:   data,
				Folder: folder,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(entry)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to GOBL document (JSON or YAML)")
	cmd.Flags().StringVar(&folder, "folder", "", "New folder for the entry")
	return cmd
}
