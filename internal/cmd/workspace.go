package cmd

import (
	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func workspaceCmd(getClient func(cmd *cobra.Command) *invopop.Client) *cobra.Command {
	return &cobra.Command{
		Use:   "workspace",
		Short: "Show current workspace info",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c := getClient(cmd)
			ws, err := c.Access().Workspace().Fetch(cmd.Context())
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(ws)
		},
	}
}
