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
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "workspace ID or slug to use after signing in")
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
	Host                    string    `json:"host"`
	Email                   string    `json:"email,omitempty"`
	UserID                  string    `json:"user_id,omitempty"`
	WorkspaceID             string    `json:"workspace_id,omitempty"`
	WorkspaceName           string    `json:"workspace_name,omitempty"`
	SessionExpiresAt        time.Time `json:"session_expires_at"`
	WorkspaceTokenExpiresAt time.Time `json:"workspace_token_expires_at,omitzero"`
}

func authStatusCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show who you are signed in as and which workspace is in use",
		RunE: func(_ *cobra.Command, _ []string) error {
			s, err := getSession()
			if err != nil {
				return err
			}
			c, err := s.Load()
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(&authStatus{
				Host:                    s.Endpoints.APIHost,
				Email:                   c.Email,
				UserID:                  c.UserID,
				WorkspaceID:             c.WorkspaceID,
				WorkspaceName:           c.WorkspaceName,
				SessionExpiresAt:        c.ExpiresAt,
				WorkspaceTokenExpiresAt: c.WorkspaceTokenExpiresAt,
			})
		},
	}
}

func authWorkspaceCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	var list bool
	cmd := &cobra.Command{
		Use:   "workspace [id-or-slug]",
		Short: "Switch to another workspace, or list them",
		Long: `Switch to another workspace, or list them.

With an ID or slug, switches to it. Without one, asks which to use at a
terminal, and prints your workspaces as JSON otherwise or with --list.`,
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
			} else if list || !term.IsTerminal(int(os.Stdin.Fd())) {
				// Agents and pipes get data rather than a prompt.
				orgs, err := s.Orgs(cmd.Context(), c)
				if err != nil {
					return err
				}
				return iohelper.PrintJSON(orgs)
			}
			ws, err := chooseWorkspace(cmd, nil, s, c, want)
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
	cmd.Flags().BoolVar(&list, "list", false, "print your workspaces as JSON instead of choosing")
	return cmd
}

func authTokenCmd(getSession func() (*auth.Session, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print a short-lived token for the current workspace",
		Long: `Print a short-lived token for the current workspace, for use with other
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
				labels[w] += "  ← current"
			}
		}
	}
	if want != "" {
		var bySlug []*auth.Workspace
		for _, w := range all {
			if w.ID == want {
				return w, nil
			}
			if strings.EqualFold(w.Slug, want) {
				bySlug = append(bySlug, w)
			}
		}
		switch len(bySlug) {
		case 1:
			return bySlug[0], nil
		case 0:
			return nil, fmt.Errorf("no workspace %q among those you belong to: run `invopop auth workspace` to list them", want)
		}
		// A live workspace and its sandbox copy can share a slug.
		msg := fmt.Sprintf("several workspaces use the slug %q, pick one by ID:", want)
		for _, w := range bySlug {
			msg += fmt.Sprintf("\n  %s  %s", w.ID, labels[w])
		}
		return nil, errors.New(msg)
	}
	switch {
	case len(all) == 0:
		return nil, errors.New("you do not belong to any workspace")
	case len(all) == 1:
		return all[0], nil
	case !term.IsTerminal(int(os.Stdin.Fd())):
		return nil, errors.New("several workspaces available: pick one with --workspace, or run `invopop auth workspace <id-or-slug>`")
	}

	cmd.PrintErrln("\nChoose a workspace:")
	for i, w := range all {
		cmd.PrintErrf("  %2d) %s  [%s]\n", i+1, labels[w], w.Slug)
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
