package entries

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

const pageSize int32 = 25

// ListModel displays a list of silo entries.
type ListModel struct {
	client  *invopop.Client
	entries []*invopop.SiloEntry
	folder  string
	cursor  int
	offset  int // scroll offset — first visible row index
	loading bool
	spinner spinner.Model
	err     string
	width   int
	height  int

	// Pagination
	nextCursor  string
	prevCursors []string
	pageNum     int
}

// NewListModel creates a new entries list page.
func NewListModel(client *invopop.Client, folder string) ListModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)
	return ListModel{
		client:  client,
		folder:  folder,
		loading: true,
		spinner: s,
		pageNum: 1,
	}
}

// Title returns the page title.
func (m ListModel) Title() string {
	if m.folder != "" {
		return folderTitle(m.folder)
	}
	return "All Entries"
}

// SetSize updates the page dimensions.
func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
}

func (m ListModel) visibleRows() int {
	r := m.height - 8 // header + table header + separator + help
	if r < 3 {
		r = 3
	}
	return r
}

// Init starts the page by fetching entries.
func (m ListModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		api.FetchEntries(m.client, m.folder, "", pageSize),
	)
}

// Update handles messages.
func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case api.EntriesFetchedMsg:
		m.loading = false
		m.entries = msg.Collection.List
		m.nextCursor = msg.Collection.NextCursor
		m.cursor = 0
		m.offset = 0
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
				m.ensureVisible()
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				m.ensureVisible()
			}
		case "g":
			m.cursor = 0
			m.offset = 0
		case "G":
			m.cursor = len(m.entries) - 1
			m.ensureVisible()
		case "enter", "l", "right":
			if m.cursor < len(m.entries) {
				entry := m.entries[m.cursor]
				return m, func() tea.Msg {
					return ui.NavigateMsg{
						Page:   "entry-detail",
						Params: map[string]string{"id": entry.ID},
					}
				}
			}
		case "n":
			if m.nextCursor != "" {
				m.loading = true
				m.prevCursors = append(m.prevCursors, m.nextCursor)
				m.pageNum++
				return m, tea.Batch(
					m.spinner.Tick,
					api.FetchEntries(m.client, m.folder, m.nextCursor, pageSize),
				)
			}
		case "p":
			if len(m.prevCursors) > 1 {
				m.loading = true
				prev := m.prevCursors[len(m.prevCursors)-2]
				m.prevCursors = m.prevCursors[:len(m.prevCursors)-1]
				m.pageNum--
				return m, tea.Batch(
					m.spinner.Tick,
					api.FetchEntries(m.client, m.folder, prev, pageSize),
				)
			} else if len(m.prevCursors) == 1 {
				m.loading = true
				m.prevCursors = nil
				m.pageNum = 1
				return m, tea.Batch(
					m.spinner.Tick,
					api.FetchEntries(m.client, m.folder, "", pageSize),
				)
			}
		case "r":
			m.loading = true
			m.cursor = 0
			m.offset = 0
			m.prevCursors = nil
			m.pageNum = 1
			return m, tea.Batch(
				m.spinner.Tick,
				api.FetchEntries(m.client, m.folder, "", pageSize),
			)
		}
	}
	return m, nil
}

// ensureVisible adjusts offset so cursor is within the visible window.
func (m *ListModel) ensureVisible() {
	vis := m.visibleRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+vis {
		m.offset = m.cursor - vis + 1
	}
}

// View renders the entries list.
func (m ListModel) View() string {
	var b strings.Builder

	// Header
	title := ui.TitleStyle.Render(m.Title())
	count := ""
	if !m.loading && len(m.entries) > 0 {
		count = ui.SubtitleStyle.Render(fmt.Sprintf("  page %d · %d entries", m.pageNum, len(m.entries)))
	}
	b.WriteString(title + count)
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(m.spinner.View() + " Loading...")
		return m.withHelp(b.String(), false)
	}

	if m.err != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(ui.Danger).Render("✗ " + m.err))
		return m.withHelp(b.String(), false)
	}

	if len(m.entries) == 0 {
		b.WriteString(ui.SubtitleStyle.Render("No entries found."))
		return m.withHelp(b.String(), false)
	}

	// Column widths
	w := m.width - 4
	colKey := 20
	colSchema := 16
	colState := 12
	colDate := 10
	colID := w - colKey - colSchema - colState - colDate - 8
	if colID < 12 {
		colID = 12
	}

	// Table header
	hdr := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
		colID, "ID", colKey, "KEY", colSchema, "SCHEMA", colState, "STATE", colDate, "CREATED")
	b.WriteString(ui.TableHeaderStyle.Render(hdr))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(ui.Border).Render(strings.Repeat("─", w)))
	b.WriteString("\n")

	// Visible rows
	vis := m.visibleRows()
	end := m.offset + vis
	if end > len(m.entries) {
		end = len(m.entries)
	}

	// Scroll indicator top
	if m.offset > 0 {
		b.WriteString(ui.SubtitleStyle.Render(fmt.Sprintf("  ↑ %d more", m.offset)))
		b.WriteString("\n")
	}

	for i := m.offset; i < end; i++ {
		entry := m.entries[i]
		id := truncate(entry.ID, colID-1)
		key := truncate(entry.Key, colKey-1)
		schema := shortSchema(entry.DocSchema)
		schema = truncate(schema, colSchema-1)
		state := entry.State
		if state == "" {
			state = "draft"
		}
		created := ""
		if len(entry.CreatedAt) >= 10 {
			created = entry.CreatedAt[:10]
		}

		row := fmt.Sprintf(" %-*s %-*s %-*s %-*s %-*s",
			colID, id, colKey, key, colSchema, schema, colState, state, colDate, created)

		if i == m.cursor {
			b.WriteString(ui.TableSelectedStyle.Width(w).Render(row))
		} else {
			b.WriteString(ui.TableRowStyle.Render(row))
		}
		b.WriteString("\n")
	}

	// Scroll indicator bottom
	remaining := len(m.entries) - end
	if remaining > 0 {
		b.WriteString(ui.SubtitleStyle.Render(fmt.Sprintf("  ↓ %d more", remaining)))
		b.WriteString("\n")
	}

	return m.withHelp(b.String(), true)
}

func (m ListModel) withHelp(content string, showNav bool) string {
	var entries []string
	entries = append(entries, ui.HelpEntry("enter", "view"))
	if showNav {
		if m.nextCursor != "" {
			entries = append(entries, ui.HelpEntry("n", "next page"))
		}
		if len(m.prevCursors) > 0 {
			entries = append(entries, ui.HelpEntry("p", "prev page"))
		}
	}
	entries = append(entries,
		ui.HelpEntry("r", "refresh"),
		ui.HelpEntry("/", "search"),
		ui.HelpEntry("esc", "back"),
	)

	help := "\n" + ui.HelpBar(entries...)
	return content + help
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

func shortSchema(schema string) string {
	parts := strings.Split(schema, "/")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "/" + parts[len(parts)-1]
	}
	return schema
}

func folderTitle(slug string) string {
	for _, f := range ui.DefaultFolders {
		if f.Slug == slug {
			return f.Name
		}
		for _, c := range f.Children {
			if c.Slug == slug {
				return c.Name
			}
		}
	}
	if len(slug) > 0 {
		return strings.ToUpper(slug[:1]) + slug[1:]
	}
	return slug
}
