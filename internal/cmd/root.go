// Package cmd contains all CLI command definitions.
package cmd

import (
	"os"

	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

// Root creates and returns the root command with all subcommands.
func Root() *cobra.Command {
	var (
		token  string
		host   string
		format string
	)

	root := &cobra.Command{
		Use:           "invocli",
		Short:         "Invopop CLI - manage your Invopop account",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&token, "token", "t", "", "API token (or INVOPOP_TOKEN env var)")
	root.PersistentFlags().StringVar(&host, "host", "", "API host (default: https://api.invopop.com)")
	root.PersistentFlags().StringVarP(&format, "format", "o", "json", "Output format: json (default), table")

	// Build client lazily so subcommands can access it.
	// Reads token from flag first, then env var.
	getClient := func(_ *cobra.Command) *invopop.Client {
		t := token
		if t == "" {
			t = os.Getenv("INVOPOP_TOKEN")
		}
		var opts []invopop.ClientOption
		if host != "" {
			opts = append(opts, invopop.WithConfig(&invopop.Config{BaseURL: host}))
		}
		if t != "" {
			opts = append(opts, invopop.WithAuthToken(t))
		}
		return invopop.New(opts...)
	}

	root.AddCommand(pingCmd(getClient))
	root.AddCommand(workspaceCmd(getClient))
	root.AddCommand(siloCmd(getClient, &format))
	root.AddCommand(transformCmd(getClient, &format))
	root.AddCommand(sequenceCmd(getClient, &format))
	root.AddCommand(skillCmd())

	return root
}
