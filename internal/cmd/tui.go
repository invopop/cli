package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/invopop/cli/internal/tui/app"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

func tuiCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the Invopop terminal UI",
		Long: "Launch the interactive Invopop terminal UI for browsing silo entries,\n" +
			"workflows, and jobs.\n\n" +
			"If no --token flag or INVOPOP_TOKEN env var is set, the TUI will prompt\n" +
			"for a token and save it under the user's config directory.",
		RunE: func(_ *cobra.Command, _ []string) error {
			// Try to build a client from CLI flags/env. If that fails (no token
			// configured), let the TUI prompt for one itself.
			client, _ := getClient()

			p := tea.NewProgram(app.New(client), tea.WithAltScreen())
			_, err := p.Run()
			return err
		},
	}
}
