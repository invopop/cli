package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Navigation pages
const (
	PageEntries   = "entries"
	PageWorkflows = "workflows"
	PageJobs      = "jobs"
	PageSearch    = "search"
)

// NavigateMsg tells the app to switch pages.
type NavigateMsg struct {
	Page   string
	Params map[string]string
}

// FolderItem represents a discovered silo folder.
type FolderItem struct {
	Slug      string
	Name      string
	DocSchema string
}

// Sidebar is the navigation sidebar.
type Sidebar struct {
	width     int
	height    int
	cursor    int
	items     []sidebarItem
	workspace string
	sandbox   bool
	focused   bool
}

type sidebarItem struct {
	label    string
	page     string
	params   map[string]string
	isHeader bool
	indent   int
}

// NewSidebar creates a new sidebar with the default folder hierarchy.
func NewSidebar() Sidebar {
	s := Sidebar{}
	s.rebuildItems()
	return s
}

// SetFocused sets whether the sidebar has keyboard focus.
func (s *Sidebar) SetFocused(focused bool) {
	s.focused = focused
}

// Focused returns whether the sidebar has keyboard focus.
func (s Sidebar) Focused() bool {
	return s.focused
}

// SetWorkspace updates the workspace shown at the top, marking a sandbox.
func (s *Sidebar) SetWorkspace(name string, sandbox bool) {
	s.workspace = name
	s.sandbox = sandbox
}

// SetSize updates the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Sidebar) rebuildItems() {
	s.items = nil

	s.items = append(s.items, sidebarItem{label: "SILO", isHeader: true})
	s.items = append(s.items, sidebarItem{
		label: "All Entries",
		page:  PageEntries,
	})
	for _, folder := range DefaultFolders {
		s.items = append(s.items, sidebarItem{
			label:  folder.Name,
			page:   PageEntries,
			params: map[string]string{"folder": folder.Slug},
		})
		for _, child := range folder.Children {
			s.items = append(s.items, sidebarItem{
				label:  child.Name,
				page:   PageEntries,
				params: map[string]string{"folder": child.Slug},
				indent: 1,
			})
		}
	}

	s.items = append(s.items, sidebarItem{label: "WORKFLOWS", isHeader: true})
	s.items = append(s.items, sidebarItem{label: "Workflows", page: PageWorkflows})
	s.items = append(s.items, sidebarItem{label: "Jobs", page: PageJobs})

	// Place cursor on first selectable
	for i, item := range s.items {
		if !item.isHeader {
			s.cursor = i
			break
		}
	}
}

// Update handles key events for the sidebar.
func (s *Sidebar) Update(msg tea.KeyMsg) tea.Cmd {
	if !s.focused {
		return nil
	}
	switch msg.String() {
	case "up", "k":
		for i := s.cursor - 1; i >= 0; i-- {
			if !s.items[i].isHeader {
				s.cursor = i
				break
			}
		}
	case "down", "j":
		for i := s.cursor + 1; i < len(s.items); i++ {
			if !s.items[i].isHeader {
				s.cursor = i
				break
			}
		}
	case "enter", "l", "right":
		if s.cursor >= 0 && s.cursor < len(s.items) && !s.items[s.cursor].isHeader {
			item := s.items[s.cursor]
			return func() tea.Msg {
				return NavigateMsg{Page: item.page, Params: item.params}
			}
		}
	}
	return nil
}

// View renders the sidebar.
func (s Sidebar) View() string {
	var b strings.Builder

	// Workspace header
	if s.workspace != "" {
		wsStyle := lipgloss.NewStyle().
			Foreground(White).
			Bold(true).
			PaddingLeft(2)
		b.WriteString(wsStyle.Render(s.workspace))
		if s.sandbox {
			b.WriteString(" " + lipgloss.NewStyle().Foreground(Warning).Render("sandbox"))
		}
		b.WriteString("\n")
	}

	for i, item := range s.items {
		if item.isHeader {
			b.WriteString(SidebarHeaderStyle.Render(item.label))
			b.WriteString("\n")
			continue
		}

		label := item.label
		selected := i == s.cursor

		if selected && s.focused {
			indicator := "▸ "
			if item.indent > 0 {
				indicator = "  ▸ "
			}
			b.WriteString(SidebarSelectedStyle.Render(fmt.Sprintf("%s%s", indicator, label)))
		} else if selected && !s.focused {
			// Selected but not focused — dim highlight
			style := lipgloss.NewStyle().
				Foreground(Green).
				PaddingLeft(2)
			if item.indent > 0 {
				style = style.PaddingLeft(4)
			}
			b.WriteString(style.Render(label))
		} else {
			style := SidebarItemStyle
			if item.indent > 0 {
				style = style.PaddingLeft(4)
			}
			b.WriteString(style.Render(label))
		}
		b.WriteString("\n")
	}

	borderColor := Border
	if s.focused {
		borderColor = Green
	}

	// Build the border manually to avoid lipgloss default color leaking
	content := b.String()
	inner := lipgloss.NewStyle().
		Width(s.width - 1). // leave room for border char
		Height(s.height).
		Render(content)

	// Add a right border column
	borderChar := "│"
	lines := strings.Split(inner, "\n")
	var result strings.Builder
	bStyle := lipgloss.NewStyle().Foreground(borderColor)
	for i, line := range lines {
		result.WriteString(line)
		result.WriteString(bStyle.Render(borderChar))
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}
