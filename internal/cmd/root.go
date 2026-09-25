// Package cmd contains all CLI command definitions.
package cmd

import (
	"os"

	"github.com/invopop/cli/internal/auth"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

const defaultAPIHost = "https://api.invopop.com"

// clientIDs holds the public OAuth client ID registered for the CLI in each
// environment, keyed by API host. A client ID is not a secret, so it ships in
// the binary; --client-id or INVOPOP_CLIENT_ID override it.
var clientIDs = map[string]string{
	// TODO: fill in once the CLI's public OAuth apps are registered, each
	// with the redirect URL urn:ietf:wg:oauth:2.0:oob.
	"https://api.invopop.com":         "5vMS_y3ThQ8Xqg",
	"https://api.invopop-staging.com": "5vMS_y3ThQ8Xqg",
	"https://api.invopop.dev":         "5vMS_y3ThQ8Xqg",
}

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
		token      string
		host       string
		accessHost string
		clientID   string
	)

	root := &cobra.Command{
		Use:           "invopop",
		Short:         "Invopop CLI - manage your Invopop account",
		Long:          banner + "\nInvopop CLI - manage your Invopop account from the terminal.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVarP(&token, "token", "t", "", "API token (or INVOPOP_TOKEN env var); overrides `invopop auth login`")
	root.PersistentFlags().StringVar(&host, "host", "", "API host (or INVOPOP_HOST env var, default: https://api.invopop.com)")
	root.PersistentFlags().StringVar(&accessHost, "access-host", "", "access service host for sign-in (or INVOPOP_ACCESS_HOST; default derived from --host)")
	root.PersistentFlags().StringVar(&clientID, "client-id", "", "OAuth client ID for sign-in (or INVOPOP_CLIENT_ID)")

	apiHost := func() string {
		return firstNonEmpty(flagOrEnv(host, "INVOPOP_HOST"), defaultAPIHost)
	}

	getSession := func() (*auth.Session, error) {
		store, err := auth.DefaultStore()
		if err != nil {
			return nil, err
		}
		h := apiHost()
		ep := &auth.Endpoints{
			APIHost:    h,
			AccessHost: firstNonEmpty(flagOrEnv(accessHost, "INVOPOP_ACCESS_HOST"), auth.AccessHostFor(h)),
			ClientID:   firstNonEmpty(flagOrEnv(clientID, "INVOPOP_CLIENT_ID"), clientIDs[h]),
		}
		return &auth.Session{Store: store, Endpoints: ep}, nil
	}

	// Build client lazily so subcommands can access it. An explicit token
	// (flag, then env var) wins; otherwise use the workspace token from a
	// stored `invopop auth login`, renewing it as needed.
	getClient := func() (*invopop.Client, error) {
		t := flagOrEnv(token, "INVOPOP_TOKEN")
		if t == "" {
			s, err := getSession()
			if err != nil {
				return nil, err
			}
			if t, err = s.WorkspaceToken(root.Context()); err != nil {
				return nil, err
			}
		}
		// Same host as the session, so a sign-in is always used where it
		// was issued.
		return invopop.New(
			invopop.WithConfig(&invopop.Config{BaseURL: apiHost()}),
			invopop.WithAuthToken(t),
		), nil
	}

	root.AddCommand(pingCmd(getClient))
	root.AddCommand(workspaceCmd(getClient))
	root.AddCommand(siloCmd(getClient))
	root.AddCommand(transformCmd(getClient))
	root.AddCommand(sequenceCmd(getClient))
	root.AddCommand(skillCmd())
	root.AddCommand(authCmd(getSession))

	return root
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// flagOrEnv returns the flag value, falling back to the named env var.
func flagOrEnv(flag, env string) string {
	if flag != "" {
		return flag
	}
	return os.Getenv(env)
}
