// Package cmd contains all CLI command definitions.
package cmd

import (
	"fmt"
	"os"

	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

const banner = `
    _
   (_)___ _   ______  ____  ____  ____
  / / __ \ | / / __ \/ __ \/ __ \/ __ \
 / / / / / |/ / /_/ / /_/ / /_/ / /_/ /
/_/_/ /_/|___/\____/ .___/\____/ .___/
                  /_/         /_/
`

// Root creates and returns the root command with all subcommands.
func Root() *cobra.Command {
	var (
		token string
		host  string
	)

	root := &cobra.Command{
		Use:           "invopop",
		Short:         "Invopop CLI - manage your Invopop account",
		Long:          banner + "\nInvopop CLI - manage your Invopop account from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&token, "token", "t", "", "API token (or INVOPOP_TOKEN env var)")
	root.PersistentFlags().StringVar(&host, "host", "", "API host (default: https://api.invopop.com)")

	// Build client lazily so subcommands can access it.
	// Reads token from flag first, then env var.
	getClient := func() (*invopop.Client, error) {
		t := token
		if t == "" {
			t = os.Getenv("INVOPOP_TOKEN")
		}
		if t == "" {
			return nil, fmt.Errorf("API token required: set --token flag or INVOPOP_TOKEN environment variable")
		}
		var opts []invopop.ClientOption
		if host != "" {
			opts = append(opts, invopop.WithConfig(&invopop.Config{BaseURL: host}))
		}
		opts = append(opts, invopop.WithAuthToken(t))
		return invopop.New(opts...), nil
	}

	root.AddCommand(pingCmd(getClient))
	root.AddCommand(workspaceCmd(getClient))
	root.AddCommand(siloCmd(getClient))
	root.AddCommand(transformCmd(getClient))
	root.AddCommand(sequenceCmd(getClient))
	root.AddCommand(skillCmd())

	return root
}
