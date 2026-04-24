package workflows

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

// ListModel displays a list of workflows.
type ListModel struct {
	client    *invopop.Client
	workflows []*invopop.Workflow
	cursor    int
	loading   bool
	spinner   spinner.Model
	err       string
	width     int
	height    int
}

// NewListModel creates a new workflows list page.
func NewListModel(client *invopop.Client) ListModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)
	return ListModel{
		client:  client,
		loading: true,
		spinner: s,
	}
}

// Title returns the page title.
func (m ListModel) Title() string { return "Workflows" }

// SetSize updates the page dimensions.
func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// Init starts the page.
func (m ListModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		api.FetchWorkflows(m.client, "", 50),
	)
}

// Update handles messages.
func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case api.WorkflowsFetchedMsg:
		m.loading = false
		m.workflows = msg.Collection.List
		return m, nil

	case api.ErrorMsg:
		m.loading = false
		m.err = msg.Error()
		return m, nil

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.loading {
			return m, nil
		}
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.workflows)-1 {
				m.cursor++
			}
		case "enter", "l", "right":
			if m.cursor < len(m.workflows) {
				wf := m.workflows[m.cursor]
				return m, func() tea.Msg {
					return ui.NavigateMsg{
						Page:   "workflow-detail",
						Params: map[string]string{"id": wf.ID},
					}
				}
			}
		case "r":
			m.loading = true
			m.cursor = 0
			return m, tea.Batch(
				m.spinner.Tick,
				api.FetchWorkflows(m.client, "", 50),
			)
		}
	}
	return m, nil
}

// View renders the workflow list.
func (m ListModel) View() string {
	var b strings.Builder

	b.WriteString(ui.TitleStyle.Render("Workflows"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + " Loading...")
		return b.String()
	}

	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("✗ " + m.err))
		return b.String()
	}

	if len(m.workflows) == 0 {
		b.WriteString(ui.SubtitleStyle.Render("No workflows found."))
		return b.String()
	}

	w := m.width - 4
	colName := 28
	colSchema := 16
	colCountry := 8
	colVersion := 10
	colSteps := 6

	hdr := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
		colName, "NAME", colSchema, "SCHEMA", colCountry, "COUNTRY", colVersion, "VERSION", colSteps, "STEPS")
	b.WriteString(ui.TableHeaderStyle.Render(hdr))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ui.Border).Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	maxRows := m.height - 8
	if maxRows < 5 {
		maxRows = 5
	}
	for i, wf := range m.workflows {
		if i >= maxRows {
			break
		}
		name := truncate(wf.Name, colName-1)
		version := wf.Version
		if wf.Draft {
			version = "draft"
		}
		steps := fmt.Sprintf("%d", len(wf.Steps))

		row := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
			colName, name, colSchema, wf.Schema, colCountry, wf.Country, colVersion, version, colSteps, steps)

		if i == m.cursor {
			b.WriteString(ui.TableSelectedStyle.Width(w).Render(row))
		} else {
			b.WriteString(ui.TableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(ui.HelpBar(
		ui.HelpEntry("enter", "view"),
		ui.HelpEntry("r", "refresh"),
		ui.HelpEntry("esc", "back"),
	))

	return b.String()
}

func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
