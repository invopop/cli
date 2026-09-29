package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/invopop/cli/internal/auth"
	iohelper "github.com/invopop/cli/internal/io"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// defaultScope asks for everything; the access service narrows the workspace
// token to what the user may actually do in the workspace they choose.
const defaultScope = "admin"

func authCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Sign in with your Invopop account instead of an API key",
		Long: `Sign in with your Invopop account instead of an API key.

"invopop auth login" opens your browser to sign in and asks you to paste back
the code it shows, then which workspace to work in. Commands afterwards use a short-lived token restricted to that
workspace, carrying no more than your own permissions there, and renew it
automatically. An explicit --token or INVOPOP_TOKEN still takes precedence.`,
	}
	cmd.AddCommand(authLoginCmd(getSession))
	cmd.AddCommand(authLogoutCmd(getSession))
	cmd.AddCommand(authStatusCmd(getSession))
	cmd.AddCommand(authWorkspaceCmd(getSession))
	cmd.AddCommand(authTokenCmd(getSession))
	return cmd
}

func authLoginCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	var (
		workspace string
		scope     string
		noBrowser bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Sign in through the browser and choose a workspace",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := getSession()
			if err != nil {
				return err
			}
			// One reader for both prompts, so neither swallows the other's input.
			in := bufio.NewReader(cmd.InOrStdin())
			opts := &auth.LoginOptions{
				Scope:  scope,
				Notify: func(msg string) { cmd.PrintErrln(msg) },
				ReadCode: func() (string, error) {
					cmd.PrintErr("Paste the code shown in your browser: ")
					return in.ReadString('\n')
				},
			}
			if !noBrowser {
				opts.OpenBrowser = openBrowser
			}
			c, err := auth.Login(cmd.Context(), s.Endpoints, opts)
			if err != nil {
				return err
			}
			if err := s.Save(c); err != nil {
				return err
			}
			cmd.PrintErrf("Signed in as %s.\n", c.Email)

			ws, err := chooseWorkspace(cmd, in, s, c, workspace)
			if err != nil {
				return err
			}
			if err := s.SelectWorkspace(cmd.Context(), c, ws); err != nil {
				return err
			}
			cmd.PrintErrf("Using workspace %s (%s).\n", ws.Name, ws.ID)
			return nil
		},
	}
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "workspace ID or slug to make the default after signing in")
	cmd.Flags().StringVar(&scope, "scope", defaultScope, "OAuth scopes to request")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "only print the sign-in URL, e.g. over SSH")
	return cmd
}

func authLogoutCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Forget the stored sign-in for this host",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := getSession()
			if err != nil {
				return err
			}
			if err := s.Store.Delete(s.Endpoints.APIHost); err != nil {
				return err
			}
			cmd.PrintErrln("Signed out. To revoke the authorization itself, remove it from your account settings.")
			return nil
		},
	}
}

type authStatus struct {
	Host   string `json:"host"`
	Email  string `json:"email,omitempty"`
	UserID string `json:"user_id,omitempty"`
	// The workspace commands act in, and what chose it.
	WorkspaceID     string `json:"workspace_id,omitempty"`
	WorkspaceName   string `json:"workspace_name,omitempty"`
	WorkspaceRef    string `json:"workspace_ref,omitempty"`
	WorkspaceSource string `json:"workspace_source"`
	// Shown only when a command-line or env override is in use.
	DefaultWorkspaceID   string `json:"default_workspace_id,omitempty"`
	DefaultWorkspaceName string `json:"default_workspace_name,omitempty"`

	AccessTokenExpiresAt    time.Time `json:"access_token_expires_at"`
	RefreshUntil            time.Time `json:"refresh_until,omitzero"`
	WorkspaceTokenExpiresAt time.Time `json:"workspace_token_expires_at,omitzero"`
}

func authStatusCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show who you are signed in as and which workspace is in use",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := getSession()
			if err != nil {
				return err
			}
			c, err := s.Load()
			if err != nil {
				return err
			}
			ws, err := s.Current(cmd.Context(), c)
			if err != nil {
				return err
			}
			st := &authStatus{
				Host:                    s.Endpoints.APIHost,
				Email:                   c.Email,
				UserID:                  c.UserID,
				WorkspaceID:             ws.ID,
				WorkspaceName:           ws.Name,
				WorkspaceRef:            ws.Ref(),
				WorkspaceSource:         workspaceSource(cmd),
				AccessTokenExpiresAt:    c.ExpiresAt,
				RefreshUntil:            c.RefreshUntil(),
				WorkspaceTokenExpiresAt: c.TokenExpiry(ws.ID),
			}
			if s.Workspace != "" {
				st.DefaultWorkspaceID = c.WorkspaceID
				st.DefaultWorkspaceName = c.WorkspaceName
			}
			return iohelper.PrintJSON(st)
		},
	}
}

// workspaceSource says what picked the workspace a command uses.
func workspaceSource(cmd *cobra.Command) string {
	if f := cmd.Flags().Lookup("workspace"); f != nil && f.Changed {
		return "--workspace"
	}
	if os.Getenv(workspaceEnv) != "" {
		return workspaceEnv
	}
	return "default"
}

func authWorkspaceCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspace [id-or-slug]",
		Short: "Choose the default workspace to work in",
		Long: `Choose the default workspace to work in.

Shows a picker, or switches straight to the ID or slug given; name a sandbox
workspace as sandbox/<slug>. To list them as JSON, e.g. for scripts or
agents, use ` + "`invopop access workspaces`" + `.

The default applies to every terminal. To use another workspace for a single
command pass --workspace, or set ` + workspaceEnv + ` for a whole terminal.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := getSession()
			if err != nil {
				return err
			}
			c, err := s.Load()
			if err != nil {
				return err
			}
			want := ""
			if len(args) > 0 {
				want = args[0]
			}
			ws, err := chooseWorkspace(cmd, nil, s, c, want)
			if err != nil {
				return err
			}
			if err := s.SelectWorkspace(cmd.Context(), c, ws); err != nil {
				return err
			}
			cmd.PrintErrf("Default workspace is now %s (%s).\n", ws.Name, ws.ID)
			if v := os.Getenv(workspaceEnv); v != "" {
				cmd.PrintErrf("%s=%s still overrides it in this terminal.\n", workspaceEnv, v)
			}
			return nil
		},
	}
	return cmd
}

func authTokenCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print a short-lived token for the workspace in use",
		Long: `Print a short-lived token for the workspace in use, for use with other
tools such as curl. It is restricted to the workspace and expires within 30
minutes; run this again for a fresh one.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := getSession()
			if err != nil {
				return err
			}
			t, err := s.WorkspaceToken(cmd.Context())
			if err != nil {
				return err
			}
			cmd.Println(t)
			return nil
		},
	}
}

// chooseWorkspace picks the workspace matching want (an ID or slug). With no
// preference it takes the only workspace there is, or asks at a terminal.
func chooseWorkspace(cmd *cobra.Command, in *bufio.Reader, s *auth.Session, c *auth.Credentials, want string) (*auth.Workspace, error) {
	if want != "" {
		return s.ResolveWorkspace(cmd.Context(), c, want)
	}
	orgs, err := s.Orgs(cmd.Context(), c)
	if err != nil {
		return nil, err
	}
	var all []*auth.Workspace
	labels := make(map[*auth.Workspace]string)
	for _, o := range orgs {
		for _, w := range o.Workspaces {
			all = append(all, w)
			labels[w] = fmt.Sprintf("%s / %s", o.Name, w.Name)
			if w.Sandbox {
				labels[w] += " (sandbox)"
			}
			if w.ID == c.WorkspaceID {
				labels[w] += "  ← default"
			}
		}
	}
	switch {
	case len(all) == 0:
		return nil, errors.New("you do not belong to any workspace")
	case len(all) == 1:
		return all[0], nil
	case !term.IsTerminal(int(os.Stdin.Fd())):
		return nil, errors.New("several workspaces available and no terminal to choose in: pass an ID or slug, or list them with `invopop access workspaces`")
	}

	cmd.PrintErrln("\nChoose a workspace:")
	for i, w := range all {
		cmd.PrintErrf("  %2d) %s  [%s]\n", i+1, labels[w], w.Ref())
	}
	if in == nil {
		in = bufio.NewReader(cmd.InOrStdin())
	}
	for {
		cmd.PrintErr("Number: ")
		line, err := in.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading choice: %w", err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && n >= 1 && n <= len(all) {
			return all[n-1], nil
		}
		cmd.PrintErrf("Enter a number from 1 to %d.\n", len(all))
	}
}

func openBrowser(url string) error {
	var c *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		c = exec.Command("open", url)
	case "windows":
		c = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		c = exec.Command("xdg-open", url)
	}
	return c.Start()
}
