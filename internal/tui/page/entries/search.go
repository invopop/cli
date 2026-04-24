package entries

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

// SearchModel handles entry search.
type SearchModel struct {
	client  *invopop.Client
	input   textinput.Model
	results []*invopop.SiloEntry
	cursor  int
	loading bool
	spinner spinner.Model
	err     string
	width   int
	height  int
	active  bool
}

// NewSearchModel creates a new search page.
func NewSearchModel(client *invopop.Client) SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Search entries..."
	ti.CharLimit = 200
	ti.Width = 40
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ui.Green)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.Green)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.White)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.FaintText)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	return SearchModel{
		client:  client,
		input:   ti,
		spinner: s,
	}
}

// Title returns the page title.
func (m SearchModel) Title() string { return "Search" }

// SetSize updates dimensions.
func (m *SearchModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.input.Width = min(w-10, 60)
}

// Activate shows and focuses the search input.
func (m *SearchModel) Activate() {
	m.active = true
	m.input.Focus()
}

// Deactivate hides the search.
func (m *SearchModel) Deactivate() {
	m.active = false
	m.input.Blur()
	m.input.SetValue("")
	m.results = nil
	m.err = ""
}

// Active returns whether search is active.
func (m SearchModel) Active() bool {
	return m.active
}

// Init starts the page.
func (m SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages.
func (m SearchModel) Update(msg tea.Msg) (SearchModel, tea.Cmd) {
	switch msg := msg.(type) {
	case api.SearchResultsMsg:
		m.loading = false
		m.results = msg.Collection.List
		m.cursor = 0
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
		switch msg.String() {
		case "esc":
			m.Deactivate()
			return m, nil
		case "enter":
			if m.input.Focused() && m.input.Value() != "" {
				m.loading = true
				m.input.Blur()
				return m, tea.Batch(
					m.spinner.Tick,
					api.SearchEntries(m.client, m.input.Value(), "", 20, 0),
				)
			}
			if !m.input.Focused() && m.cursor < len(m.results) {
				entry := m.results[m.cursor]
				m.Deactivate()
				return m, func() tea.Msg {
					return ui.NavigateMsg{
						Page:   "entry-detail",
						Params: map[string]string{"id": entry.ID},
					}
				}
			}
		case "up", "k":
			if !m.input.Focused() && m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if !m.input.Focused() && m.cursor < len(m.results)-1 {
				m.cursor++
			}
		case "/":
			m.input.Focus()
			return m, textinput.Blink
		}
	}

	if m.input.Focused() {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// View renders the search page.
func (m SearchModel) View() string {
	if !m.active {
		return ""
	}

	var b strings.Builder

	b.WriteString(ui.TitleStyle.Render("Search"))
	b.WriteString("\n\n")
	b.WriteString(ui.InputLabelStyle.Render("Query: "))
	b.WriteString(m.input.View())
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + " Searching...")
		return b.String()
	}

	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("✗ " + m.err))
		return b.String()
	}

	if len(m.results) == 0 && m.input.Value() != "" && !m.input.Focused() {
		b.WriteString(ui.SubtitleStyle.Render("No results found."))
		return b.String()
	}

	for i, entry := range m.results {
		id := truncate(entry.ID, 12)
		key := truncate(entry.Key, 20)
		schema := shortSchema(entry.DocSchema)
		folder := entry.Folder

		row := fmt.Sprintf(" %-14s %-22s %-16s %s", id, key, schema, folder)

		if i == m.cursor {
			b.WriteString(ui.TableSelectedStyle.Width(m.width - 4).Render(row))
		} else {
			b.WriteString(ui.TableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(ui.HelpBar(
		ui.HelpEntry("enter", "select"),
		ui.HelpEntry("/", "search again"),
		ui.HelpEntry("esc", "close"),
	))

	return b.String()
}
