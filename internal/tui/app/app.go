// Package app hosts the root bubble tea model that wires together TUI pages,
// sidebar navigation, and token/connection state.
package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/config"
	"github.com/invopop/cli/internal/tui/page/entries"
	"github.com/invopop/cli/internal/tui/page/jobs"
	"github.com/invopop/cli/internal/tui/page/workflows"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

type state int

const (
	stateTokenInput state = iota
	stateConnecting
	stateReady
)

// page identifies the active content page.
type page int

const (
	pageEntriesList page = iota
	pageEntryDetail
	pageSearch
	pageWorkflowsList
	pageWorkflowDetail
	pageJobs
)

// Model is the root application model.
type Model struct {
	state     state
	client    *invopop.Client
	workspace *invopop.Workspace
	cfg       *config.Config

	// UI components
	sidebar ui.Sidebar
	spinner spinner.Model

	// Token input
	tokenInput textinput.Model

	// Active page
	activePage  page
	entryList   entries.ListModel
	entryDetail entries.DetailModel
	search      entries.SearchModel
	wfList      workflows.ListModel
	wfDetail    workflows.DetailModel
	jobsView    jobs.ListModel

	// Sidebar focused
	sidebarFocused bool

	// Dimensions
	width  int
	height int

	// Error/message
	err string
}

// New creates a new application model. If client is non-nil the TUI skips the
// token prompt and connects immediately; otherwise it falls back to reading
// the token from env/config or prompting the user.
func New(client *invopop.Client) Model {
	cfg := config.Load()

	ti := textinput.New()
	ti.Placeholder = "Paste your API token..."
	ti.CharLimit = 500
	ti.Width = 60
	ti.EchoMode = textinput.EchoPassword
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(ui.Green)
	ti.PromptStyle = lipgloss.NewStyle().Foreground(ui.Green)
	ti.TextStyle = lipgloss.NewStyle().Foreground(ui.White)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.FaintText)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	m := Model{
		cfg:            cfg,
		sidebar:        ui.NewSidebar(),
		tokenInput:     ti,
		spinner:        s,
		sidebarFocused: true,
	}
	m.sidebar.SetFocused(true)

	switch {
	case client != nil:
		m.state = stateConnecting
		m.client = client
	case cfg.Token != "":
		m.state = stateConnecting
		m.client = m.buildClient(cfg.Token)
	default:
		m.state = stateTokenInput
		m.tokenInput.Focus()
	}

	return m
}

// Init starts the application.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{tea.EnterAltScreen}

	if m.state == stateConnecting {
		cmds = append(cmds, m.spinner.Tick, api.FetchWorkspace(m.client))
	} else {
		cmds = append(cmds, textinput.Blink)
	}

	return tea.Batch(cmds...)
}

func (m *Model) buildClient(token string) *invopop.Client {
	var opts []invopop.ClientOption
	if m.cfg.Host != "" {
		opts = append(opts, invopop.WithConfig(&invopop.Config{BaseURL: m.cfg.Host}))
	}
	opts = append(opts, invopop.WithAuthToken(token))
	return invopop.New(opts...)
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateReady {
			m.updateSizes()
		}
		return m, nil

	case tea.KeyMsg:
		// Global keys
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.state == stateTokenInput {
				return m, tea.Quit
			}
			// Quit unless actively editing or typing in an input
			if m.state == stateReady && !m.isEditing() && !m.search.Active() {
				return m, tea.Quit
			}
		}
	}

	switch m.state {
	case stateTokenInput:
		return m.updateTokenInput(msg)
	case stateConnecting:
		return m.updateConnecting(msg)
	case stateReady:
		return m.updateReady(msg)
	}

	return m, nil
}

func (m Model) updateTokenInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			token := strings.TrimSpace(m.tokenInput.Value())
			if token == "" {
				return m, nil
			}
			if err := config.SaveToken(token); err != nil {
				m.err = "failed to save token: " + err.Error()
			}
			m.client = m.buildClient(token)
			m.state = stateConnecting
			return m, tea.Batch(m.spinner.Tick, api.FetchWorkspace(m.client))
		}
	}

	var cmd tea.Cmd
	m.tokenInput, cmd = m.tokenInput.Update(msg)
	return m, cmd
}

func (m Model) updateConnecting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case api.WorkspaceFetchedMsg:
		m.workspace = msg.Workspace
		m.state = stateReady
		m.sidebar.SetWorkspace(msg.Workspace.Name)

		// Start with entries list
		m.activePage = pageEntriesList
		m.entryList = entries.NewListModel(m.client, "")
		m.search = entries.NewSearchModel(m.client)
		m.updateSizes()
		return m, m.entryList.Init()

	case api.ErrorMsg:
		// Token was bad, go back to token input
		m.state = stateTokenInput
		m.tokenInput.Focus()
		m.tokenInput.SetValue("")
		m.err = "Authentication failed: " + msg.Error()
		return m, textinput.Blink

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateReady(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle navigation messages
	if nav, ok := msg.(ui.NavigateMsg); ok {
		return m.navigate(nav)
	}

	// Handle keyboard
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// Search toggle
		if keyMsg.String() == "/" && !m.search.Active() && m.activePage == pageEntriesList {
			m.search.Activate()
			m.search.SetSize(m.contentWidth(), m.contentHeight())
			m.sidebarFocused = false
			m.sidebar.SetFocused(false)
			return m, m.search.Init()
		}

		// Tab between sidebar and content
		if keyMsg.String() == "tab" && !m.search.Active() {
			if m.activePage != pageWorkflowDetail {
				m.sidebarFocused = !m.sidebarFocused
				m.sidebar.SetFocused(m.sidebarFocused)
				return m, nil
			}
		}

		// Escape from content back to sidebar
		if keyMsg.String() == "esc" && !m.sidebarFocused {
			if m.search.Active() {
				m.search.Deactivate()
				return m, nil
			}
			switch m.activePage {
			case pageEntryDetail:
				m.activePage = pageEntriesList
				return m, nil
			case pageWorkflowDetail:
				m.activePage = pageWorkflowsList
				return m, nil
			default:
				m.sidebarFocused = true
				m.sidebar.SetFocused(true)
				return m, nil
			}
		}

		// Route keys to sidebar or content
		if m.sidebarFocused {
			cmd := m.sidebar.Update(keyMsg)
			return m, cmd
		}
	}

	// Route to active search
	if m.search.Active() {
		var cmd tea.Cmd
		m.search, cmd = m.search.Update(msg)
		return m, cmd
	}

	// Route to active page
	return m.updateActivePage(msg)
}

func (m Model) updateActivePage(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch m.activePage {
	case pageEntriesList:
		m.entryList, cmd = m.entryList.Update(msg)
	case pageEntryDetail:
		m.entryDetail, cmd = m.entryDetail.Update(msg)
	case pageWorkflowsList:
		m.wfList, cmd = m.wfList.Update(msg)
	case pageWorkflowDetail:
		m.wfDetail, cmd = m.wfDetail.Update(msg)
	case pageJobs:
		m.jobsView, cmd = m.jobsView.Update(msg)
	}

	return m, cmd
}

func (m Model) navigate(nav ui.NavigateMsg) (tea.Model, tea.Cmd) {
	m.sidebarFocused = false
	m.sidebar.SetFocused(false)
	m.err = ""

	switch nav.Page {
	case ui.PageEntries:
		folder := nav.Params["folder"]
		m.activePage = pageEntriesList
		m.entryList = entries.NewListModel(m.client, folder)
		m.updateSizes()
		return m, m.entryList.Init()

	case "entry-detail":
		id := nav.Params["id"]
		m.activePage = pageEntryDetail
		m.entryDetail = entries.NewDetailModel(m.client, id)
		m.updateSizes()
		return m, m.entryDetail.Init()

	case ui.PageWorkflows:
		m.activePage = pageWorkflowsList
		m.wfList = workflows.NewListModel(m.client)
		m.updateSizes()
		return m, m.wfList.Init()

	case "workflow-detail":
		id := nav.Params["id"]
		m.activePage = pageWorkflowDetail
		m.wfDetail = workflows.NewDetailModel(m.client, id)
		m.updateSizes()
		return m, m.wfDetail.Init()

	case ui.PageJobs:
		jobID := nav.Params["job_id"]
		if jobID != "" {
			m.activePage = pageJobs
			m.jobsView = jobs.NewListModelWithJob(m.client, jobID)
		} else {
			m.activePage = pageJobs
			m.jobsView = jobs.NewListModel(m.client)
		}
		m.updateSizes()
		return m, m.jobsView.Init()
	}

	return m, nil
}

// isEditing returns true if the user is in a text editing mode.
func (m Model) isEditing() bool {
	return (m.activePage == pageEntryDetail && m.entryDetail.Editing()) ||
		(m.activePage == pageWorkflowDetail && m.wfDetail.Editing()) ||
		(m.activePage == pageJobs && m.jobsView.InputFocused())
}

func (m *Model) updateSizes() {
	sidebarW := ui.SidebarWidth + 2
	contentW := m.width - sidebarW
	contentH := m.height - 3 // status bar

	m.sidebar.SetSize(sidebarW, contentH)
	switch m.activePage {
	case pageEntriesList:
		m.entryList.SetSize(contentW, contentH)
	case pageEntryDetail:
		m.entryDetail.SetSize(contentW, contentH)
	case pageWorkflowsList:
		m.wfList.SetSize(contentW, contentH)
	case pageWorkflowDetail:
		m.wfDetail.SetSize(contentW, contentH)
	case pageJobs:
		m.jobsView.SetSize(contentW, contentH)
	}
	m.search.SetSize(contentW, contentH)
}

func (m Model) contentWidth() int {
	return m.width - ui.SidebarWidth - 2
}

func (m Model) contentHeight() int {
	return m.height - 3
}

// View renders the full application.
func (m Model) View() string {
	switch m.state {
	case stateTokenInput:
		return m.viewTokenInput()
	case stateConnecting:
		return m.viewConnecting()
	case stateReady:
		return m.viewReady()
	}
	return ""
}

func (m Model) viewTokenInput() string {
	var b strings.Builder

	title := lipgloss.NewStyle().
		Foreground(ui.Primary).
		Bold(true).
		MarginBottom(1).
		Render("invotui — Invopop Terminal UI")

	b.WriteString("\n\n")
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, title))
	b.WriteString("\n\n")

	prompt := "Enter your Invopop API token to connect.\n" +
		"You can also set the INVOPOP_TOKEN environment variable.\n\n"
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, prompt))
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, m.tokenInput.View()))

	if m.err != "" {
		b.WriteString("\n\n")
		errMsg := lipgloss.NewStyle().Foreground(ui.Danger).Render(m.err)
		b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, errMsg))
	}

	b.WriteString("\n\n")
	help := ui.HelpBar(ui.HelpEntry("enter", "connect"), ui.HelpEntry("q", "quit"))
	b.WriteString(lipgloss.PlaceHorizontal(m.width, lipgloss.Center, help))

	return b.String()
}

func (m Model) viewConnecting() string {
	content := m.spinner.View() + " Connecting to Invopop..."
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewReady() string {
	sidebar := m.sidebar.View()

	// Render active content
	var content string
	if m.search.Active() {
		content = m.search.View()
	} else {
		switch m.activePage {
		case pageEntriesList:
			content = m.entryList.View()
		case pageEntryDetail:
			content = m.entryDetail.View()
		case pageWorkflowsList:
			content = m.wfList.View()
		case pageWorkflowDetail:
			content = m.wfDetail.View()
		case pageJobs:
			content = m.jobsView.View()
		}
	}

	contentArea := ui.ContentStyle.
		Width(m.contentWidth()).
		Height(m.contentHeight()).
		Render(content)

	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, contentArea)

	// Footer help bar
	help := ui.HelpBar(
		ui.HelpEntry("tab", "focus"),
		ui.HelpEntry("/", "search"),
		ui.HelpEntry("q", "quit"),
	)
	divider := lipgloss.NewStyle().Foreground(ui.Border).Render(strings.Repeat("─", m.width))
	footer := divider + "\n " + help

	return lipgloss.JoinVertical(lipgloss.Left, main, footer)
}
