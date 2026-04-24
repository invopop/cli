// Package workflows implements the transform-workflows TUI pages (list, detail).
package workflows

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/component"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

// DetailModel shows a single workflow with visual step rendering or JSON editing.
type DetailModel struct {
	client     *invopop.Client
	workflowID string
	workflow   *invopop.Workflow
	editor     component.JSONEditor
	viewport   viewport.Model
	viewMode   viewMode // visual or json
	editing    bool
	loading    bool
	saving     bool
	spinner    spinner.Model
	message    string
	err        string
	width      int
	height     int
}

type viewMode int

const (
	viewVisual viewMode = iota
	viewJSON
)

// NewDetailModel creates a new workflow detail page.
func NewDetailModel(client *invopop.Client, workflowID string) DetailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Foreground(ui.OffWhite)
	return DetailModel{
		client:     client,
		workflowID: workflowID,
		editor:     component.NewJSONEditor(),
		viewport:   vp,
		loading:    true,
		spinner:    s,
	}
}

// Title returns the page title.
func (m DetailModel) Title() string {
	if m.workflow != nil {
		return fmt.Sprintf("Workflow: %s", m.workflow.Name)
	}
	return "Workflow"
}

// SetSize updates dimensions.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.editor.SetSize(w-4, h-10)
	m.viewport.Width = w - 4
	m.viewport.Height = h - 10
}

// Editing returns whether the JSON editor is active.
func (m DetailModel) Editing() bool { return m.editing }

// Init starts the page.
func (m DetailModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		api.FetchWorkflow(m.client, m.workflowID),
	)
}

// Update handles messages.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case api.WorkflowFetchedMsg:
		m.loading = false
		m.workflow = msg.Workflow
		m.loadEditorContent()
		m.viewport.SetContent(m.renderSteps())
		m.viewport.GotoTop()
		return m, nil

	case api.WorkflowUpdatedMsg:
		m.saving = false
		m.workflow = msg.Workflow
		m.message = "Workflow saved successfully"
		m.err = ""
		m.loadEditorContent()
		m.viewport.SetContent(m.renderSteps())
		return m, nil

	case api.ErrorMsg:
		m.loading = false
		m.saving = false
		m.err = msg.Error()
		return m, nil

	case spinner.TickMsg:
		if m.loading || m.saving {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	case tea.KeyMsg:
		if m.loading || m.saving {
			return m, nil
		}

		// JSON editor mode
		if m.viewMode == viewJSON && m.editing && m.editor.Focused() {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.editor.SetReadOnly(true)
				m.editor.Blur()
				return m, nil
			case "ctrl+s":
				return m, m.save()
			default:
				cmd := m.editor.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "tab":
			if m.viewMode == viewVisual {
				m.viewMode = viewJSON
			} else {
				m.viewMode = viewVisual
			}
			return m, nil
		case "e":
			if m.viewMode == viewJSON && !m.editing {
				m.editing = true
				m.editor.SetReadOnly(false)
				m.editor.Focus()
				m.message = ""
			}
		}

		// In visual mode, let viewport handle scrolling
		if m.viewMode == viewVisual {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}

		// In JSON read-only mode, forward to editor for viewport scrolling
		if m.viewMode == viewJSON && !m.editing {
			cmd := m.editor.Update(msg)
			return m, cmd
		}
	}

	if m.viewMode == viewJSON && m.editing && m.editor.Focused() {
		cmd := m.editor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *DetailModel) loadEditorContent() {
	if m.workflow == nil {
		return
	}
	data, err := json.Marshal(m.workflow)
	if err != nil {
		return
	}
	m.editor.SetContent(data)
	m.editor.SetReadOnly(true)
}

func (m *DetailModel) save() tea.Cmd {
	data, err := m.editor.Content()
	if err != nil {
		m.err = err.Error()
		return nil
	}

	var wf invopop.Workflow
	if err := json.Unmarshal(data, &wf); err != nil {
		m.err = "invalid workflow JSON: " + err.Error()
		return nil
	}

	m.saving = true
	m.message = ""
	return tea.Batch(
		m.spinner.Tick,
		api.UpdateWorkflow(m.client, &invopop.UpdateWorkflow{
			ID:          m.workflowID,
			Name:        wf.Name,
			Description: wf.Description,
			Steps:       wf.Steps,
			Rescue:      wf.Rescue,
		}),
	)
}

// View renders the workflow detail.
func (m DetailModel) View() string {
	var b strings.Builder

	if m.loading {
		b.WriteString(m.spinner.View() + " Loading workflow...")
		return b.String()
	}

	if m.workflow == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("Workflow not found"))
		return b.String()
	}

	// Header
	header := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Muted).
		Padding(0, 1).
		Width(m.width - 4)

	version := m.workflow.Version
	if m.workflow.Draft {
		version = "draft"
	}
	disabled := ""
	if m.workflow.Disabled {
		disabled = "  [DISABLED]"
	}

	info := fmt.Sprintf(
		"%s%s\n%s\nSchema: %s  Country: %s  Version: %s  Steps: %d",
		m.workflow.Name, disabled,
		m.workflow.Description,
		m.workflow.Schema, m.workflow.Country, version, len(m.workflow.Steps),
	)
	b.WriteString(header.Render(info))
	b.WriteString("\n\n")

	// Status
	if m.saving {
		b.WriteString(m.spinner.View() + " Saving...\n")
	}
	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("Error: "+m.err) + "\n")
	}
	if m.message != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Success).Render(m.message) + "\n")
	}

	// View mode indicator
	viewLabel := "Visual"
	if m.viewMode == viewJSON {
		viewLabel = "JSON"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).Render(
		fmt.Sprintf("[%s]  tab: switch view", viewLabel),
	))
	b.WriteString("\n\n")

	if m.viewMode == viewVisual {
		b.WriteString(m.viewport.View())
	} else {
		if m.editing {
			b.WriteString(lipgloss.NewStyle().Foreground(ui.Warning).Render("EDITING") + "\n")
		}
		b.WriteString(m.editor.View())
	}

	b.WriteString("\n")

	// Help
	var hints []string
	if m.viewMode == viewJSON {
		if m.editing {
			hints = append(hints, "ctrl+s: save", "esc: stop editing")
		} else {
			hints = append(hints, "e: edit", "tab: visual view")
		}
	} else {
		hints = append(hints, "j/k: scroll", "tab: json view")
	}
	hints = append(hints, "esc: back")
	var helpEntries []string
	for _, h := range hints {
		parts := strings.SplitN(h, ": ", 2)
		if len(parts) == 2 {
			helpEntries = append(helpEntries, ui.HelpEntry(parts[0], parts[1]))
		}
	}
	b.WriteString(ui.HelpBar(helpEntries...))

	return b.String()
}

func (m DetailModel) renderSteps() string {
	if m.workflow == nil {
		return ""
	}

	var b strings.Builder
	maxWidth := m.width - 8
	if maxWidth < 30 {
		maxWidth = 30
	}

	// Main steps
	if len(m.workflow.Steps) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Primary).Bold(true).Render("Steps"))
		b.WriteString("\n\n")
		for i, step := range m.workflow.Steps {
			b.WriteString(renderStepBox(step, maxWidth, false))
			if i < len(m.workflow.Steps)-1 {
				b.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).PaddingLeft(3).Render("│"))
				b.WriteString("\n")
				b.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).PaddingLeft(3).Render("▼"))
				b.WriteString("\n")
			}
		}
	}

	// Rescue steps
	if len(m.workflow.Rescue) > 0 {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Bold(true).Render("Rescue Steps"))
		b.WriteString("\n\n")
		for i, step := range m.workflow.Rescue {
			b.WriteString(renderStepBox(step, maxWidth, true))
			if i < len(m.workflow.Rescue)-1 {
				b.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).PaddingLeft(3).Render("│"))
				b.WriteString("\n")
				b.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).PaddingLeft(3).Render("▼"))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

func renderStepBox(step *invopop.Step, maxWidth int, isRescue bool) string {
	style := ui.StepBoxStyle.Width(maxWidth)
	if isRescue {
		style = ui.RescueBoxStyle.Width(maxWidth)
	}

	name := step.Name
	if name == "" {
		name = step.ID
	}

	var content strings.Builder

	// Step name with action
	title := lipgloss.NewStyle().Bold(true).Render(name)
	if step.Action != "" {
		title += lipgloss.NewStyle().Foreground(ui.Muted).Render(
			fmt.Sprintf("  (%s)", step.Action),
		)
	}
	content.WriteString(title)

	// Notes
	if step.Notes != "" {
		content.WriteString("\n")
		content.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).Italic(true).Render(step.Notes))
	}

	// Next step routing
	if len(step.Next) > 0 {
		content.WriteString("\n")
		for _, next := range step.Next {
			label := next.StepID
			if next.Stop {
				label = lipgloss.NewStyle().Foreground(ui.Danger).Render("STOP")
			}
			var conditions []string
			if next.Status != "" {
				conditions = append(conditions, next.Status)
			}
			if next.Code != "" {
				conditions = append(conditions, "code:"+next.Code)
			}
			cond := ""
			if len(conditions) > 0 {
				cond = lipgloss.NewStyle().Foreground(ui.Warning).Render(
					fmt.Sprintf(" [%s]", strings.Join(conditions, ", ")),
				)
			}
			content.WriteString(lipgloss.NewStyle().Foreground(ui.Secondary).Render(
				fmt.Sprintf("  → %s", label),
			))
			content.WriteString(cond)
			content.WriteString("\n")
		}
	}

	return style.Render(content.String()) + "\n"
}
