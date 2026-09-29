package component

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/ui"
)

// StatusBar renders a bottom status bar.
type StatusBar struct {
	width     int
	workspace string
	message   string
	help      string
}

// NewStatusBar creates a new status bar.
func NewStatusBar() StatusBar {
	return StatusBar{}
}

// SetWidth sets the status bar width.
func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

// SetWorkspace sets the workspace display name.
func (s *StatusBar) SetWorkspace(name string) {
	s.workspace = name
}

// SetMessage sets the center message.
func (s *StatusBar) SetMessage(msg string) {
	s.message = msg
}

// SetHelp sets the right-aligned help text.
func (s *StatusBar) SetHelp(help string) {
	s.help = help
}

// View renders the status bar.
func (s StatusBar) View() string {
	w := s.width
	if w < 10 {
		w = 80
	}

	ws := lipgloss.NewStyle().
		Foreground(ui.Primary).
		Bold(true).
		Render(s.workspace)

	help := lipgloss.NewStyle().
		Foreground(ui.Muted).
		Render(s.help)

	msg := lipgloss.NewStyle().
		Foreground(ui.Muted).
		Render(s.message)

	// Build content: workspace + message on left, help on right
	left := ws
	if msg != "" {
		left += "  " + msg
	}

	leftW := lipgloss.Width(left)
	rightW := lipgloss.Width(help)
	innerW := w - 2 // account for padding

	gap := innerW - leftW - rightW
	if gap < 1 {
		gap = 1
	}

	content := left + strings.Repeat(" ", gap) + help

	bar := lipgloss.NewStyle().
		Width(w).
		MaxWidth(w).
		BorderTop(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(ui.Border).
		Padding(0, 1).
		Render(content)

	return bar
}
