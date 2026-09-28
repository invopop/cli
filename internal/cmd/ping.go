package cmd

import (
	iohelper "github.com/invopop/cli/internal/io"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

func pingCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Test API connectivity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			p := &invopop.Ping{}
			if err := c.Utils().Ping(cmd.Context(), p); err != nil {
				return err
			}
			return iohelper.PrintJSON(p)
		},
	}
}
