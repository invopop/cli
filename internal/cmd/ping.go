package cmd

import (
	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func pingCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Test API connectivity",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := getClient(cmd)
			p := &invopop.Ping{}
			if err := c.Utils().Ping(cmd.Context(), p); err != nil {
				return err
			}
			return iohelper.PrintJSON(p)
		},
	}
}
