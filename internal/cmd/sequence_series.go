package cmd

import (
	"encoding/json"

	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func sequenceSeriesCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	series := &cobra.Command{
		Use:   "series",
		Short: "Manage sequence series",
	}

	series.AddCommand(sequenceSeriesListCmd(getClient))
	series.AddCommand(sequenceSeriesCreateCmd(getClient))
	series.AddCommand(sequenceSeriesFetchCmd(getClient))

	return series
}

func sequenceSeriesListCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all series",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := getClient(cmd)
			col, err := c.Sequence().List(cmd.Context())
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(col)
		},
	}
}

func sequenceSeriesCreateCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	var (
		file    string
		name    string
		prefix  string
		suffix  string
		padding int32
		start   int32
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a series",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var req invopop.CreateSeries

			if file != "" {
				data, err := iohelper.ReadInput(file)
				if err != nil {
					return err
				}
				if err := json.Unmarshal(data, &req); err != nil {
					return err
				}
			} else {
				req = invopop.CreateSeries{
					Name:    name,
					Prefix:  prefix,
					Suffix:  suffix,
					Padding: padding,
					Start:   start,
				}
			}

			c := getClient(cmd)
			s, err := c.Sequence().Create(cmd.Context(), &req)
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(s)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to series definition (JSON or YAML)")
	cmd.Flags().StringVar(&name, "name", "", "Series name")
	cmd.Flags().StringVar(&prefix, "prefix", "", "Code prefix")
	cmd.Flags().StringVar(&suffix, "suffix", "", "Code suffix")
	cmd.Flags().Int32Var(&padding, "padding", 0, "Zero-padding width")
	cmd.Flags().Int32Var(&start, "start", 0, "Starting index")
	return cmd
}

func sequenceSeriesFetchCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <id>",
		Short: "Fetch a series by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := getClient(cmd)
			s, err := c.Sequence().Fetch(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(s)
		},
	}
}
