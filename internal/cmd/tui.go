package cmd

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/invopop/cli/internal/auth"
	"github.com/invopop/cli/internal/tui/app"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

func tuiCmd(getClient func() (*invopop.Client, error), clientFor func(ref string) (*invopop.Client, error), getSession func() (*auth.Session, error), apiToken func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the Invopop terminal UI",
		Long: `Launch the interactive Invopop terminal UI for browsing silo entries,
workflows, and jobs.

It uses the same credentials as other commands: your sign-in from
` + "`invopop auth login`" + `, or an API token from --token or INVOPOP_TOKEN. It opens
in the workspace other commands would use, which --workspace changes.

Press w to switch workspace, which needs a sign-in: an API token belongs to
a single workspace.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			// Fail here, with the usual advice, rather than inside the UI.
			client, err := getClient()
			if err != nil {
				return err
			}
			opts := app.Options{Client: client, Connect: clientFor}
			if apiToken() == "" {
				opts.Workspaces = func(ctx context.Context, inUseID string) ([]*auth.WorkspaceListing, error) {
					s, c, err := loadSession(getSession)
					if err != nil {
						return nil, err
					}
					orgs, err := s.Orgs(ctx, c)
					if err != nil {
						return nil, err
					}
					return auth.ListWorkspaces(orgs, c.WorkspaceID, inUseID), nil
				}
				opts.SetDefault = func(ctx context.Context, id string) error {
					s, c, err := loadSession(getSession)
					if err != nil {
						return err
					}
					ws, err := s.ResolveWorkspace(ctx, c, id)
					if err != nil {
						return err
					}
					return s.SelectWorkspace(ctx, c, ws)
				}
			}

			p := tea.NewProgram(app.New(opts), tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
}

// loadSession returns the session and its stored credentials.
func loadSession(getSession func() (*auth.Session, error)) (*auth.Session, *auth.Credentials, error) {
	s, err := getSession()
	if err != nil {
		return nil, nil, err
	}
	c, err := s.Load()
	if err != nil {
		return nil, nil, err
	}
	return s, c, nil
}
