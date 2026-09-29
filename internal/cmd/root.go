// Package cmd contains all CLI command definitions.
package cmd

import (
	"errors"
	"os"

	"github.com/invopop/cli/internal/auth"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

const defaultAPIHost = "https://api.invopop.com"

// workspaceEnv names a workspace to use instead of the stored default, e.g.
// one per terminal or per project directory.
const workspaceEnv = "INVOPOP_WORKSPACE"

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
		workspace  string
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
	root.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "workspace ID or slug for this command only, sandbox/<slug> for a sandbox (or "+workspaceEnv+"); overrides the default set by invopop auth workspace")

	apiHost := func() string {
		// One spelling for client-ID lookup, stored sign-ins and requests.
		return auth.NormalizeHost(firstNonEmpty(flagOrEnv(host, "INVOPOP_HOST"), defaultAPIHost))
	}

	getSession := func() (*auth.Session, error) {
		store, err := auth.DefaultStore()
		if err != nil {
			return nil, err
		}
		h := apiHost()
		ep := &auth.Endpoints{
			APIHost:    h,
			AccessHost: auth.NormalizeHost(firstNonEmpty(flagOrEnv(accessHost, "INVOPOP_ACCESS_HOST"), auth.AccessHostFor(h))),
			ClientID:   firstNonEmpty(flagOrEnv(clientID, "INVOPOP_CLIENT_ID"), clientIDs[h]),
		}
		return &auth.Session{Store: store, Endpoints: ep, Workspace: flagOrEnv(workspace, workspaceEnv)}, nil
	}

	// Build client lazily so subcommands can access it. An explicit token
	// (flag, then env var) wins; otherwise use the workspace token from a
	// stored `invopop auth login`, renewing it as needed.
	apiToken := func() string { return flagOrEnv(token, "INVOPOP_TOKEN") }

	// clientFor builds a client for the workspace ref names, or for the
	// default when ref is empty. An explicit token (flag, then env var) wins;
	// otherwise the stored sign-in supplies a token for the workspace, renewed
	// on each request as needed so long-running commands keep working.
	clientFor := func(ref string) (*invopop.Client, error) {
		// Same host as the session, so a sign-in is always used where it
		// was issued.
		conf := invopop.WithConfig(&invopop.Config{BaseURL: apiHost()})
		if t := apiToken(); t != "" {
			if ref != "" {
				return nil, errors.New("--workspace or " + workspaceEnv + " cannot be used with an API token, which already belongs to one workspace")
			}
			return invopop.New(conf, invopop.WithAuthToken(t)), nil
		}
		s, err := getSession()
		if err != nil {
			return nil, err
		}
		s.Workspace = ref
		c, err := s.Load()
		if err != nil {
			return nil, err
		}
		ws, err := s.Current(root.Context(), c)
		if err != nil {
			return nil, err
		}
		// Pin the ID, so the workspace cannot change under a running
		// command, and get a token now so problems show before any request.
		s.Workspace = ws.ID
		if _, err := s.WorkspaceToken(root.Context()); err != nil {
			return nil, err
		}
		client := invopop.New(conf)
		hc := client.HTTPClient()
		hc.Transport = s.Transport(hc.Transport)
		return client, nil
	}
	getClient := func() (*invopop.Client, error) {
		return clientFor(flagOrEnv(workspace, workspaceEnv))
	}

	root.AddCommand(pingCmd(getClient))
	root.AddCommand(accessCmd(getClient, getSession, apiToken))
	root.AddCommand(siloCmd(getClient))
	root.AddCommand(transformCmd(getClient))
	root.AddCommand(sequenceCmd(getClient))
	root.AddCommand(tuiCmd(getClient, clientFor, getSession, apiToken))
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
