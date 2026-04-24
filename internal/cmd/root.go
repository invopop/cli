// Package cmd contains all CLI command definitions.
package cmd

import (
	"fmt"
	"os"

	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

// Root creates and returns the root command with all subcommands.
func Root() *cobra.Command {
	var (
		token string
		host  string
	)

	root := &cobra.Command{
		Use:           "invocli",
		Short:         "Invopop CLI - manage your Invopop account",
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
