// Package workspaces provides the workspace switcher shown over the content
// area when the user presses w.
package workspaces

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/invopop/cli/internal/auth"
	"github.com/invopop/cli/internal/tui/ui"
)

// ListFunc lists the workspaces the user belongs to, marking inUseID.
type ListFunc func(ctx context.Context, inUseID string) ([]*auth.WorkspaceListing, error)

// LoadedMsg carries the workspaces once listed.
type LoadedMsg struct {
	List []*auth.WorkspaceListing
	Err  error
}

// PickedMsg is sent when the user chooses a workspace.
type PickedMsg struct {
	Workspace *auth.WorkspaceListing
	// MakeDefault asks for the choice to become the stored default as well
	// as the workspace for this session.
	MakeDefault bool
}

// ClosedMsg is sent when the user leaves the picker without choosing.
type ClosedMsg struct{}

// PickerModel lists workspaces, filtered by what the user types.
type PickerModel struct {
	list    ListFunc
	inUseID string
	loading bool
	err     string
	all     []*auth.WorkspaceListing
	shown   []*auth.WorkspaceListing
	cursor  int
	filter  textinput.Model
	spinner spinner.Model
	width   int
	height  int
}

// NewPicker creates a picker that lists workspaces with list, marking the
// one with inUseID as current.
func NewPicker(list ListFunc, inUseID string) PickerModel {
	ti := textinput.New()
	ti.Placeholder = "Type to filter by name, org, slug or ID"
	ti.Prompt = "› "
	ti.CharLimit = 100
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ui.Green)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.Green)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.White)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.FaintText)
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	return PickerModel{list: list, inUseID: inUseID, loading: true, filter: ti, spinner: s}
}

// Init starts loading the workspaces.
func (m PickerModel) Init() tea.Cmd {
	list, inUse := m.list, m.inUseID
	load := func() tea.Msg {
		ws, err := list(context.Background(), inUse)
		return LoadedMsg{List: ws, Err: err}
	}
	return tea.Batch(load, m.spinner.Tick, textinput.Blink)
}

// SetSize sets the space the picker may use.
func (m *PickerModel) SetSize(w, h int) {
	m.width, m.height = w, h
	m.filter.Width = max(w-8, 10)
}

// Update handles loading and key presses.
func (m PickerModel) Update(msg tea.Msg) (PickerModel, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.all = msg.List
		m.applyFilter()
		// Start on the workspace in use, so enter alone changes nothing.
		for i, w := range m.shown {
			if w.InUse {
				m.cursor = i
			}
		}
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return ClosedMsg{} }
		case "up", "ctrl+p", "shift+tab":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+n", "tab":
			if m.cursor < len(m.shown)-1 {
				m.cursor++
			}
			return m, nil
		case "enter", "ctrl+d":
			if m.cursor >= len(m.shown) {
				return m, nil
			}
			picked := PickedMsg{Workspace: m.shown[m.cursor], MakeDefault: msg.String() == "ctrl+d"}
			return m, func() tea.Msg { return picked }
		}
	}

	var cmd tea.Cmd
	before := m.filter.Value()
	m.filter, cmd = m.filter.Update(msg)
	if m.filter.Value() != before {
		m.applyFilter()
	}
	return m, cmd
}

func (m *PickerModel) applyFilter() {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	m.shown = m.shown[:0]
	for _, w := range m.all {
		hay := strings.ToLower(strings.Join([]string{w.Name, w.OrgName, w.Ref, w.ID}, " "))
		if q == "" || strings.Contains(hay, q) {
			m.shown = append(m.shown, w)
		}
	}
	if m.cursor >= len(m.shown) {
		m.cursor = max(len(m.shown)-1, 0)
	}
}

// View renders the picker.
func (m PickerModel) View() string {
	var b strings.Builder
	b.WriteString(ui.TitleStyle.Render("Switch workspace"))
	b.WriteString("\n\n")
	b.WriteString(m.filter.View())
	b.WriteString("\n\n")

	switch {
	case m.loading:
		b.WriteString(m.spinner.View() + " Loading workspaces…")
	case m.err != "":
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render(m.err))
	case len(m.shown) == 0:
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).Render("No workspace matches."))
	default:
		b.WriteString(m.rows())
	}

	b.WriteString("\n\n")
	b.WriteString(ui.HelpBar(
		ui.HelpEntry("enter", "switch"),
		ui.HelpEntry("ctrl+d", "make default"),
		ui.HelpEntry("esc", "cancel"),
	))
	return b.String()
}

// rows renders the list under a heading per org, scrolled to keep the
// cursor in view and truncated to fit the width. The workspace in use is
// marked with a dot.
func (m PickerModel) rows() string {
	width := max(m.width-5, 24) // content padding
	const defaultTag = "  default"
	nameW, refW := 0, 0
	for _, w := range m.shown {
		nameW = max(nameW, lipgloss.Width(w.Name))
		refW = max(refW, lipgloss.Width(w.Ref))
	}
	// Cursor, dot and the gaps take 6 cells; the name gets up to 60% of
	// what is left after the tag, the ref the rest.
	avail := width - 6 - len(defaultTag)
	nameW = min(nameW, max(avail*3/5, avail-refW))
	refW = min(refW, avail-nameW)

	muted := lipgloss.NewStyle().Foreground(ui.Muted)
	var lines []string
	cursorLine := 0
	org := "\x00"
	for i, w := range m.shown {
		if w.OrgName != org {
			org = w.OrgName
			lines = append(lines, muted.Render(pad(org, width)))
		}
		dot := "  "
		if w.InUse {
			dot = lipgloss.NewStyle().Foreground(ui.Green).Render("●") + " "
		}
		refStyle := muted
		if w.Sandbox {
			refStyle = lipgloss.NewStyle().Foreground(ui.Warning)
		}
		line := dot + pad(w.Name, nameW) + "  " + refStyle.Render(pad(w.Ref, refW))
		if w.Default {
			line += muted.Render(defaultTag)
		}
		if i == m.cursor {
			cursorLine = len(lines)
			line = ui.SidebarSelectedStyle.UnsetPaddingLeft().Render("▸ ") + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	room := max(m.height-9, 3) // title, filter, help and spacing
	if len(lines) <= room {
		return strings.Join(lines, "\n")
	}
	room-- // for the line saying how many more there are
	start := max(min(cursorLine-room/2, len(lines)-room), 0)
	end := start + room
	out := lines[start:end]
	if more := len(lines) - end; more > 0 {
		out = append(out, muted.Render(fmt.Sprintf("  … %d more lines", more)))
	} else {
		out = append(out, "")
	}
	return strings.Join(out, "\n")
}

// pad fits s to exactly w cells, cutting it short with an ellipsis.
func pad(s string, w int) string {
	if lipgloss.Width(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	return s + strings.Repeat(" ", w-lipgloss.Width(s))
}
