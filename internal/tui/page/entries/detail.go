// Package entries implements the silo-entries TUI pages (list, detail, search).
package entries

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/component"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

// DetailModel shows a single silo entry with editing.
type DetailModel struct {
	client  *invopop.Client
	entryID string
	entry   *invopop.SiloEntry
	editor  component.JSONEditor
	editing bool
	loading bool
	saving  bool
	spinner spinner.Model
	message string
	err     string
	width   int
	height  int
}

// NewDetailModel creates a new entry detail page.
func NewDetailModel(client *invopop.Client, entryID string) DetailModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)
	return DetailModel{
		client:  client,
		entryID: entryID,
		editor:  component.NewJSONEditor(),
		loading: true,
		spinner: s,
	}
}

// Title returns the page title.
func (m DetailModel) Title() string {
	return fmt.Sprintf("Entry: %s", truncate(m.entryID, 20))
}

// SetSize updates the page dimensions.
func (m *DetailModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.editor.SetSize(w-4, h-14)
}

// Editing returns whether the editor is active.
func (m DetailModel) Editing() bool { return m.editing }

// Init starts the page.
func (m DetailModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		api.FetchEntry(m.client, m.entryID),
	)
}

// Update handles messages.
func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case api.EntryFetchedMsg:
		m.loading = false
		m.entry = msg.Entry
		if m.entry.Data != nil {
			m.editor.SetContent(m.entry.Data)
		}
		m.editor.SetReadOnly(true)
		return m, nil

	case api.EntryUpdatedMsg:
		m.saving = false
		m.entry = msg.Entry
		m.message = "Saved"
		m.err = ""
		if m.entry.Data != nil {
			m.editor.SetContent(m.entry.Data)
		}
		return m, nil

	case api.GOBLBuiltMsg:
		m.saving = false
		m.editor.SetContent(msg.Data)
		m.message = "Built"
		m.err = ""
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
		if m.editing && m.editor.Focused() {
			switch msg.String() {
			case "esc":
				m.editing = false
				m.editor.SetReadOnly(true)
				m.editor.Blur()
				return m, nil
			case "ctrl+s":
				return m, m.save()
			case "ctrl+b":
				return m, m.build()
			default:
				cmd := m.editor.Update(msg)
				return m, cmd
			}
		}
		switch msg.String() {
		case "e":
			if !m.editing {
				m.editing = true
				m.editor.SetReadOnly(false)
				m.editor.Focus()
				m.message = ""
				return m, nil
			}
		case "ctrl+b":
			return m, m.build()
		default:
			// In read-only mode, forward j/k/up/down/pgup/pgdown to viewport for scrolling
			if !m.editing {
				cmd := m.editor.Update(msg)
				return m, cmd
			}
		}
	}

	if m.editing && m.editor.Focused() {
		cmd := m.editor.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *DetailModel) save() tea.Cmd {
	data, err := m.editor.Content()
	if err != nil {
		m.err = err.Error()
		return nil
	}
	m.saving = true
	m.message = ""
	return tea.Batch(
		m.spinner.Tick,
		api.UpdateEntry(m.client, &invopop.UpdateSiloEntry{
			ID:   m.entryID,
			Data: data,
		}),
	)
}

func (m *DetailModel) build() tea.Cmd {
	data, err := m.editor.Content()
	if err != nil {
		m.err = err.Error()
		return nil
	}
	m.saving = true
	m.message = ""
	return tea.Batch(
		m.spinner.Tick,
		api.BuildGOBL(m.client, data),
	)
}

// View renders the detail page.
func (m DetailModel) View() string {
	var b strings.Builder

	if m.loading {
		b.WriteString(m.spinner.View() + " Loading entry...")
		return b.String()
	}

	if m.entry == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("Entry not found"))
		return b.String()
	}

	// Info header
	schema := shortSchema(m.entry.DocSchema)
	state := m.entry.State
	if state == "" {
		state = "draft"
	}

	stateBadge := ui.BadgeState.Render(state)
	if m.entry.Signed {
		stateBadge = ui.BadgeSigned.Render("signed")
	}

	row1 := lipgloss.JoinHorizontal(lipgloss.Center,
		ui.InputLabelStyle.Render("ID ")+m.entry.ID,
		"   ",
		stateBadge,
	)
	row2 := lipgloss.JoinHorizontal(lipgloss.Center,
		ui.SubtitleStyle.Render("Key: ")+m.entry.Key,
		"   ",
		ui.SubtitleStyle.Render("Schema: ")+schema,
		"   ",
		ui.SubtitleStyle.Render("Folder: ")+m.entry.Folder,
	)

	header := ui.InfoBoxStyle.Width(m.width - 4).Render(row1 + "\n" + row2)
	b.WriteString(header)
	b.WriteString("\n\n")

	// Status messages
	if m.saving {
		b.WriteString(m.spinner.View() + " Processing...\n")
	}
	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("✗ "+m.err) + "\n")
	}
	if m.message != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Success).Render("✓ "+m.message) + "\n")
	}

	// Mode indicator
	if m.editing {
		b.WriteString(ui.BadgeDraft.Render(" EDITING ") + "\n")
	}
	b.WriteString(m.editor.View())

	// Help footer
	b.WriteString("\n")
	var entries []string
	if m.editing {
		entries = append(entries,
			ui.HelpEntry("ctrl+s", "save"),
			ui.HelpEntry("ctrl+b", "build"),
			ui.HelpEntry("esc", "stop editing"),
		)
	} else {
		entries = append(entries,
			ui.HelpEntry("e", "edit"),
			ui.HelpEntry("ctrl+b", "build"),
			ui.HelpEntry("esc", "back"),
		)
	}
	b.WriteString(ui.HelpBar(entries...))

	return b.String()
}
