package cmd

import (
	"encoding/json"

	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func siloBuildCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	var (
		file    string
		envelop bool
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build/validate a GOBL document",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := iohelper.ReadInput(file)
			if err != nil {
				return err
			}
			c := getClient(cmd)
			result, err := c.Silo().GOBL().Build(cmd.Context(), &invopop.BuildGOBL{
				Data:    json.RawMessage(data),
				Envelop: envelop,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintRawJSON(result.Data)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to GOBL document (JSON or YAML)")
	cmd.Flags().BoolVar(&envelop, "envelop", false, "Wrap document in an envelope")
	return cmd
}

func siloSignCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "sign",
		Short: "Sign a GOBL document",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := iohelper.ReadInput(file)
			if err != nil {
				return err
			}
			c := getClient(cmd)
			result, err := c.Silo().GOBL().Sign(cmd.Context(), &invopop.SignGOBL{
				Data: json.RawMessage(data),
			})
			if err != nil {
				return err
			}
			return iohelper.PrintRawJSON(result.Data)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to GOBL envelope (JSON or YAML)")
	return cmd
}

func siloCmd(getClient func(cmd *cobra.Command) *invopop.Client, format *string) *cobra.Command {
	silo := &cobra.Command{
		Use:   "silo",
		Short: "Manage silo entries, files, states, and GOBL documents",
	}

	silo.AddCommand(siloEntriesCmd(getClient, format))
	silo.AddCommand(siloFilesCmd(getClient))
	silo.AddCommand(siloBuildCmd(getClient))
	silo.AddCommand(siloSignCmd(getClient))

	return silo
}
