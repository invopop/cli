// Package app hosts the root bubble tea model that wires together TUI pages,
// sidebar navigation, and the connection to a workspace.
package app

import (
	"context"
	"reflect"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/page/entries"
	"github.com/invopop/cli/internal/tui/page/jobs"
	"github.com/invopop/cli/internal/tui/page/workflows"
	"github.com/invopop/cli/internal/tui/page/workspaces"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

type state int

const (
	stateConnecting state = iota
	stateReady
	stateFailed
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

// Options connect the TUI to the CLI's credentials.
type Options struct {
	// Client is for the workspace the TUI opens in.
	Client *invopop.Client
	// Connect builds a client for the workspace with the given ID.
	Connect func(id string) (*invopop.Client, error)
	// Workspaces lists those the user can switch to. Nil when an API token
	// is in use, as it reaches only one workspace.
	Workspaces workspaces.ListFunc
	// SetDefault makes the workspace with the given ID the stored default.
	SetDefault func(ctx context.Context, id string) error
}

// Model is the root application model.
type Model struct {
	state     state
	opts      Options
	client    *invopop.Client
	workspace *invopop.Workspace

	// gen counts workspace switches. Commands are tagged with the gen they
	// were started in, so a response arriving after a switch is dropped
	// rather than shown against the new workspace.
	gen int

	// UI components
	sidebar ui.Sidebar
	spinner spinner.Model

	// Workspace switcher, shown over the content while picking.
	picker    workspaces.PickerModel
	picking   bool
	switching string // name of the workspace being switched to

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

// New creates the application model for the workspace opts.Client is for.
func New(opts Options) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ui.Primary)

	m := Model{
		state:          stateConnecting,
		opts:           opts,
		client:         opts.Client,
		sidebar:        ui.NewSidebar(),
		spinner:        s,
		sidebarFocused: true,
	}
	m.sidebar.SetFocused(true)
	return m
}

// Init starts the application.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, m.spinner.Tick, scoped(m.gen, api.FetchWorkspace(m.client)))
}

// scopedMsg wraps a message with the gen of the command that produced it.
type scopedMsg struct {
	gen int
	msg tea.Msg
}

// teaPkg is bubbletea's package path. Its own messages, such as quitting,
// are passed on as they are rather than scoped.
var teaPkg = reflect.TypeOf(tea.QuitMsg{}).PkgPath()

// scoped tags what cmd produces with gen, including each command of a batch.
func scoped(gen int, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		msg := cmd()
		switch v := msg.(type) {
		case nil:
			return nil
		case tea.BatchMsg:
			out := make(tea.BatchMsg, len(v))
			for i, c := range v {
				out[i] = scoped(gen, c)
			}
			return out
		}
		if reflect.TypeOf(msg).PkgPath() == teaPkg {
			return msg
		}
		return scopedMsg{gen: gen, msg: msg}
	}
}

// switchedMsg reports a new workspace connected and loaded.
type switchedMsg struct {
	client    *invopop.Client
	workspace *invopop.Workspace
}

// switchTo connects to the workspace with id, making it the default first
// when asked, and loads its details before anything is replaced, so a
// failure leaves the current workspace in place.
func (m Model) switchTo(id string, makeDefault bool) tea.Cmd {
	opts := m.opts
	return func() tea.Msg {
		ctx := context.Background()
		if makeDefault {
			if err := opts.SetDefault(ctx, id); err != nil {
				return api.ErrorMsg{Err: err, Context: "set default workspace"}
			}
		}
		client, err := opts.Connect(id)
		if err != nil {
			return api.ErrorMsg{Err: err, Context: "switch workspace"}
		}
		ws, err := client.Access().Workspace().Fetch(ctx)
		if err != nil {
			return api.ErrorMsg{Err: err, Context: "switch workspace"}
		}
		return switchedMsg{client: client, workspace: ws}
	}
}

// enter makes ws the workspace in view, starting again from its entries.
func (m Model) enter(client *invopop.Client, ws *invopop.Workspace) (Model, tea.Cmd) {
	m.client = client
	m.workspace = ws
	m.state = stateReady
	m.switching = ""
	m.picking = false
	m.sidebar = ui.NewSidebar()
	m.sidebar.SetWorkspace(ws.Name, ws.Sandbox)
	m.sidebarFocused = true
	m.sidebar.SetFocused(true)

	m.activePage = pageEntriesList
	m.entryList = entries.NewListModel(m.client, "")
	m.search = entries.NewSearchModel(m.client)
	m.updateSizes()
	return m, m.entryList.Init()
}

// Update handles messages, dropping any left over from before a workspace
// switch and tagging the commands it starts with the current gen.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sm, ok := msg.(scopedMsg); ok {
		if sm.gen != m.gen {
			return m, nil
		}
		msg = sm.msg
	}
	next, cmd := m.update(msg)
	nm := next.(Model)
	return nm, scoped(nm.gen, cmd)
}

func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.state == stateFailed {
				return m, tea.Quit
			}
			// Quit unless actively editing or typing in an input
			if m.state == stateReady && !m.picking && !m.isEditing() && !m.search.Active() {
				return m, tea.Quit
			}
		}
	}

	switch m.state {
	case stateConnecting:
		return m.updateConnecting(msg)
	case stateReady:
		return m.updateReady(msg)
	}

	return m, nil
}

func (m Model) updateConnecting(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case api.WorkspaceFetchedMsg:
		return m.enter(m.client, msg.Workspace)

	case api.ErrorMsg:
		m.state = stateFailed
		m.err = msg.Error()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m Model) updateReady(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Workspace switching
	switch msg := msg.(type) {
	case switchedMsg:
		m.gen++ // anything still in flight belongs to the old workspace
		m.err = ""
		return m.enter(msg.client, msg.workspace)
	case workspaces.PickedMsg:
		m.picking = false
		if msg.Workspace.InUse && !msg.MakeDefault {
			return m, nil
		}
		m.switching = msg.Workspace.Name
		return m, tea.Batch(m.spinner.Tick, m.switchTo(msg.Workspace.ID, msg.MakeDefault))
	case workspaces.ClosedMsg:
		m.picking = false
		return m, nil
	case api.ErrorMsg:
		if m.switching != "" {
			m.switching = ""
			m.err = msg.Error()
			return m, nil
		}
	case spinner.TickMsg:
		if m.switching != "" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	if m.picking {
		var cmd tea.Cmd
		m.picker, cmd = m.picker.Update(msg)
		return m, cmd
	}

	// Handle navigation messages
	if nav, ok := msg.(ui.NavigateMsg); ok {
		return m.navigate(nav)
	}

	// Handle keyboard
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if keyMsg.String() == "w" && !m.search.Active() && !m.isEditing() {
			return m.openPicker()
		}

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

// openPicker shows the workspace switcher, when switching is possible.
func (m Model) openPicker() (tea.Model, tea.Cmd) {
	switch {
	case m.switching != "":
		return m, nil
	case m.opts.Workspaces == nil:
		m.err = "Switching workspace needs a sign-in: run invopop auth login instead of using an API token"
		return m, nil
	}
	m.err = ""
	m.picking = true
	m.picker = workspaces.NewPicker(m.opts.Workspaces, m.workspace.ID)
	m.picker.SetSize(m.contentWidth(), m.contentHeight())
	return m, m.picker.Init()
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
	m.picker.SetSize(contentW, contentH)
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
	case stateFailed:
		return m.viewFailed()
	case stateConnecting:
		return m.viewConnecting()
	case stateReady:
		return m.viewReady()
	}
	return ""
}

func (m Model) viewFailed() string {
	msg := lipgloss.NewStyle().Foreground(ui.Danger).Render("Could not connect to Invopop: " + m.err)
	help := ui.HelpBar(ui.HelpEntry("q", "quit"))
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg+"\n\n"+help)
}

func (m Model) viewConnecting() string {
	content := m.spinner.View() + " Connecting to Invopop..."
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, content)
}

func (m Model) viewReady() string {
	sidebar := m.sidebar.View()

	// Render active content
	var content string
	if m.picking {
		content = m.picker.View()
	} else if m.search.Active() {
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

	// Footer help bar, or what is happening instead
	help := []string{
		ui.HelpEntry("tab", "focus"),
		ui.HelpEntry("/", "search"),
	}
	if m.opts.Workspaces != nil {
		help = append(help, ui.HelpEntry("w", "workspace"))
	}
	help = append(help, ui.HelpEntry("q", "quit"))
	line := ui.HelpBar(help...)
	switch {
	case m.switching != "":
		line = m.spinner.View() + " Switching to " + m.switching + "…"
	case m.err != "":
		line = lipgloss.NewStyle().Foreground(ui.Danger).Render(m.err)
	}
	divider := lipgloss.NewStyle().Foreground(ui.Border).Render(strings.Repeat("─", m.width))
	footer := divider + "\n " + line

	return lipgloss.JoinVertical(lipgloss.Left, main, footer)
}
