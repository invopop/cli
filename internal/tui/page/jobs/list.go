// Package jobs implements the transform-jobs TUI page.
package jobs

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

// ListModel provides job lookup and displays job details.
type ListModel struct {
	client  *invopop.Client
	input   textinput.Model
	job     *invopop.Job
	history []*jobRef // session history of viewed jobs
	loading bool
	spinner spinner.Model
	err     string
	scroll  int
	width   int
	height  int
}

type jobRef struct {
	ID   string
	Name string // workflow name if known
}

// NewListModel creates a new jobs page.
func NewListModel(client *invopop.Client) ListModel {
	ti := textinput.New()
	ti.Placeholder = "Enter job ID..."
	ti.CharLimit = 80
	ti.Width = 40
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ui.Green)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.Green)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.White)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.FaintText)
	ti.Focus()

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	return ListModel{
		client:  client,
		input:   ti,
		spinner: s,
	}
}

// NewListModelWithJob creates a jobs page pre-loaded with a job ID.
func NewListModelWithJob(client *invopop.Client, jobID string) ListModel {
	m := NewListModel(client)
	m.input.SetValue(jobID)
	m.input.Blur()
	m.loading = true
	return m
}

// Title returns the page title.
func (m ListModel) Title() string { return "Jobs" }

// SetSize updates dimensions.
func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

// InputFocused returns whether the job ID input is focused.
func (m ListModel) InputFocused() bool { return m.input.Focused() }

// Init starts the page.
func (m ListModel) Init() tea.Cmd {
	if m.loading {
		return tea.Batch(
			m.spinner.Tick,
			api.FetchJob(m.client, m.input.Value()),
		)
	}
	return textinput.Blink
}

// Update handles messages.
func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case api.JobFetchedMsg:
		m.loading = false
		m.job = msg.Job
		m.history = append(m.history, &jobRef{ID: msg.Job.ID})
		m.scroll = 0
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

		if m.input.Focused() {
			switch msg.String() {
			case "enter":
				if m.input.Value() != "" {
					m.loading = true
					m.job = nil
					m.err = ""
					m.input.Blur()
					return m, tea.Batch(
						m.spinner.Tick,
						api.FetchJob(m.client, m.input.Value()),
					)
				}
			case "esc":
				if m.job != nil {
					m.input.Blur()
				}
				return m, nil
			default:
				var cmd tea.Cmd
				m.input, cmd = m.input.Update(msg)
				return m, cmd
			}
		} else {
			switch msg.String() {
			case "/":
				m.input.Focus()
				return m, textinput.Blink
			case "up", "k":
				if m.scroll > 0 {
					m.scroll--
				}
			case "down", "j":
				m.scroll++
			}
		}
	}

	if m.input.Focused() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the jobs page.
func (m ListModel) View() string {
	var b strings.Builder

	b.WriteString(ui.TitleStyle.Render("Jobs"))
	b.WriteString("\n\n")

	// Input
	b.WriteString("Job ID: ")
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + " Loading job...")
		return b.String()
	}

	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("Error: " + m.err))
		b.WriteString("\n\n")
	}

	if m.job != nil {
		b.WriteString(m.renderJob())
	}

	b.WriteString("\n")
	if m.input.Focused() {
		b.WriteString(ui.HelpBar(ui.HelpEntry("enter", "fetch"), ui.HelpEntry("esc", "cancel")))
	} else {
		b.WriteString(ui.HelpBar(ui.HelpEntry("/", "lookup"), ui.HelpEntry("j/k", "scroll"), ui.HelpEntry("esc", "back")))
	}

	return b.String()
}

func (m ListModel) renderJob() string {
	job := m.job
	var b strings.Builder

	// Header
	header := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Muted).
		Padding(0, 1).
		Width(m.width - 4)

	status := "running"
	statusStyle := lipgloss.NewStyle().Foreground(ui.Warning)
	if job.CompletedAt != "" {
		if len(job.Faults) > 0 {
			status = "completed with errors"
			statusStyle = lipgloss.NewStyle().Foreground(ui.Danger)
		} else {
			status = "completed"
			statusStyle = lipgloss.NewStyle().Foreground(ui.Success)
		}
	}

	info := fmt.Sprintf(
		"Job: %s\nWorkflow: %s  Entry: %s\nStatus: %s  Created: %s",
		job.ID, job.WorkflowID, job.SiloEntryID,
		statusStyle.Render(status), job.CreatedAt,
	)
	b.WriteString(header.Render(info))
	b.WriteString("\n\n")

	// Faults
	if len(job.Faults) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Bold(true).Render("Faults"))
		b.WriteString("\n")
		for _, f := range job.Faults {
			b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render(
				fmt.Sprintf("  • [%s] %s: %s", f.Code, f.Provider, f.Message),
			))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Intents
	if len(job.Intents) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Primary).Bold(true).Render("Intents"))
		b.WriteString("\n")
		for i, intent := range job.Intents {
			if i < m.scroll {
				continue
			}
			b.WriteString(renderIntent(intent, m.width-8))
		}
	}

	return b.String()
}

func renderIntent(intent *invopop.JobIntent, maxWidth int) string {
	var b strings.Builder

	// Intent box
	style := ui.StepBoxStyle.Width(maxWidth)

	name := intent.StepID
	if intent.Provider != "" {
		name = fmt.Sprintf("%s (%s)", intent.StepID, intent.Provider)
	}

	status := "pending"
	if intent.Completed {
		status = "completed"
	}

	var content strings.Builder
	content.WriteString(lipgloss.NewStyle().Bold(true).Render(name))
	content.WriteString(lipgloss.NewStyle().Foreground(ui.Muted).Render(
		fmt.Sprintf("  [%s]", status),
	))

	// Events
	if len(intent.Events) > 0 {
		content.WriteString("\n")
		for _, evt := range intent.Events {
			evtStyle := lipgloss.NewStyle().Foreground(ui.Muted)
			marker := "○"
			switch evt.Status {
			case "ok", "OK":
				evtStyle = lipgloss.NewStyle().Foreground(ui.Success)
				marker = "●"
			case "err", "ERR", "ko", "KO":
				evtStyle = lipgloss.NewStyle().Foreground(ui.Danger)
				marker = "✗"
			case "skip", "SKIP":
				evtStyle = lipgloss.NewStyle().Foreground(ui.Warning)
				marker = "○"
			}
			line := fmt.Sprintf("  %s %s", marker, evt.Status)
			if evt.Code != "" {
				line += fmt.Sprintf(" [%s]", evt.Code)
			}
			if evt.Message != "" {
				line += ": " + evt.Message
			}
			if evt.At != "" {
				line += lipgloss.NewStyle().Foreground(ui.Muted).Render(
					fmt.Sprintf("  (%s)", evt.At),
				)
			}
			content.WriteString(evtStyle.Render(line))
			content.WriteString("\n")
		}
	}

	b.WriteString(style.Render(content.String()))
	b.WriteString("\n")
	return b.String()
}
