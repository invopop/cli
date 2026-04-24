package cmd

import (
	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/cli/internal/io"
	"github.com/spf13/cobra"
)

func workspaceCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "workspace",
		Short: "Show current workspace info",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			ws, err := c.Access().Workspace().Fetch(cmd.Context())
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(ws)
		},
	}
}
