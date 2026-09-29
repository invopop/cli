package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/invopop/cli/internal/auth"
	"github.com/invopop/cli/internal/tui/api"
	"github.com/invopop/cli/internal/tui/page/workspaces"
	"github.com/invopop/cli/internal/tui/ui"
	"github.com/invopop/client.go/invopop"
)

// fakeAPI answers the workspace lookup with the workspace a token names.
func fakeAPI(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer tok-")
		if r.URL.Path != "/access/v1/workspace" || !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(&invopop.Workspace{ID: id, Name: "Workspace " + id, Sandbox: id == "w2"})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func testOptions(t *testing.T) (Options, *[]string) {
	host := fakeAPI(t)
	connect := func(id string) (*invopop.Client, error) {
		return invopop.New(invopop.WithConfig(&invopop.Config{BaseURL: host}), invopop.WithAuthToken("tok-"+id)), nil
	}
	c, _ := connect("w1")
	var defaults []string
	return Options{
		Client:  c,
		Connect: connect,
		Workspaces: func(_ context.Context, inUse string) ([]*auth.WorkspaceListing, error) {
			return []*auth.WorkspaceListing{
				{ID: "w1", Name: "Workspace w1", Ref: "one", InUse: inUse == "w1", Default: true},
				{ID: "w2", Name: "Workspace w2", Ref: "sandbox/two", Sandbox: true, InUse: inUse == "w2"},
			}, nil
		},
		SetDefault: func(_ context.Context, id string) error {
			defaults = append(defaults, id)
			return nil
		},
	}, &defaults
}

// send delivers msg and returns the model with the message the resulting
// command produces, when it produces one straight away.
func send(t *testing.T, m Model, msg tea.Msg) (Model, tea.Msg) {
	t.Helper()
	next, cmd := m.Update(msg)
	m = next.(Model)
	if cmd == nil {
		return m, nil
	}
	return m, cmd()
}

// find unwraps msg, and the messages of a batch, for the first of type T.
func find[T any](msg tea.Msg) (T, bool) {
	var zero T
	switch v := msg.(type) {
	case scopedMsg:
		return find[T](v.msg)
	case tea.BatchMsg:
		for _, c := range v {
			if c == nil {
				continue
			}
			if got, ok := find[T](c()); ok {
				return got, true
			}
		}
	case T:
		return v, true
	}
	return zero, false
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// ready starts the TUI in w1.
func ready(t *testing.T, opts Options) Model {
	t.Helper()
	m := New(opts)
	m, _ = send(t, m, tea.WindowSizeMsg{Width: 120, Height: 40})
	m, _ = send(t, m, api.WorkspaceFetchedMsg{Workspace: &invopop.Workspace{ID: "w1", Name: "Workspace w1"}})
	if m.state != stateReady {
		t.Fatalf("state = %v", m.state)
	}
	return m
}

// pick opens the switcher, loads it, and moves to the second workspace.
func pick(t *testing.T, m Model) Model {
	t.Helper()
	m, msg := send(t, m, key("w"))
	if !m.picking {
		t.Fatalf("picker did not open: err = %q", m.err)
	}
	loaded, ok := find[workspaces.LoadedMsg](msg)
	if !ok {
		t.Fatal("picker did not load workspaces")
	}
	m, _ = send(t, m, loaded)
	// Typing q filters rather than quitting.
	if _, msg := send(t, m, key("q")); msg == tea.Quit() {
		t.Fatal("q quit while picking")
	}
	m, _ = send(t, m, key("down"))
	return m
}

func TestSwitchWorkspace(t *testing.T) {
	opts, defaults := testOptions(t)
	m := pick(t, ready(t, opts))

	m, msg := send(t, m, key("enter"))
	picked, ok := find[workspaces.PickedMsg](msg)
	if !ok || picked.Workspace.ID != "w2" {
		t.Fatalf("picked = %+v", picked)
	}
	m, msg = send(t, m, picked)
	if m.picking || m.switching != "Workspace w2" {
		t.Fatalf("picking = %v, switching = %q", m.picking, m.switching)
	}
	switched, ok := find[switchedMsg](msg)
	if !ok {
		t.Fatalf("no switch: %#v", msg)
	}

	// A response from the old workspace, still in flight.
	stale := scopedMsg{gen: m.gen, msg: api.EntriesFetchedMsg{Collection: &invopop.SiloEntryCollection{
		List: []*invopop.SiloEntry{{ID: "old-entry", Key: "old-entry", Folder: "sales", State: "sent"}},
	}}}

	m, _ = send(t, m, switched)
	if m.workspace.ID != "w2" || m.gen != 1 || m.switching != "" {
		t.Fatalf("after switch: workspace = %s, gen = %d, switching = %q", m.workspace.ID, m.gen, m.switching)
	}
	if !m.sidebar.Focused() || m.activePage != pageEntriesList {
		t.Error("switch did not start again from the entries list")
	}
	if len(*defaults) != 0 {
		t.Errorf("enter changed the default: %v", *defaults)
	}

	before := m.View()
	m, _ = send(t, m, stale)
	if m.View() != before {
		t.Error("stale response from the old workspace was applied")
	}
	// The same response in the current gen is applied, so the check above
	// does detect it.
	stale.gen = m.gen
	if m, _ = send(t, m, stale); m.View() == before {
		t.Error("current response was not applied")
	}
}

func TestSwitchAndMakeDefault(t *testing.T) {
	opts, defaults := testOptions(t)
	m := pick(t, ready(t, opts))
	_, msg := send(t, m, key("ctrl+d"))
	picked, _ := find[workspaces.PickedMsg](msg)
	if !picked.MakeDefault {
		t.Fatal("ctrl+d did not ask for a default")
	}
	_, msg = send(t, m, picked)
	if _, ok := find[switchedMsg](msg); !ok {
		t.Fatalf("no switch: %#v", msg)
	}
	if len(*defaults) != 1 || (*defaults)[0] != "w2" {
		t.Errorf("defaults set = %v", *defaults)
	}
}

func TestSwitchFailureKeepsWorkspace(t *testing.T) {
	opts, _ := testOptions(t)
	opts.Connect = func(string) (*invopop.Client, error) { return nil, context.DeadlineExceeded }
	m := pick(t, ready(t, opts))
	_, msg := send(t, m, key("enter"))
	picked, _ := find[workspaces.PickedMsg](msg)
	m, msg = send(t, m, picked)
	fail, ok := find[api.ErrorMsg](msg)
	if !ok {
		t.Fatalf("expected an error, got %#v", msg)
	}
	m, _ = send(t, m, fail)
	if m.workspace.ID != "w1" || m.switching != "" || !strings.Contains(m.err, "switch workspace") {
		t.Fatalf("workspace = %s, switching = %q, err = %q", m.workspace.ID, m.switching, m.err)
	}
}

func TestNoSwitchingWithAPIToken(t *testing.T) {
	opts, _ := testOptions(t)
	opts.Workspaces = nil
	m := ready(t, opts)
	m, _ = send(t, m, key("w"))
	if m.picking || !strings.Contains(m.err, "needs a sign-in") {
		t.Fatalf("picking = %v, err = %q", m.picking, m.err)
	}
	m.err = ""
	if strings.Contains(m.View(), ui.HelpEntry("w", "workspace")) {
		t.Error("footer offers w without a sign-in")
	}
}

func TestScopedPassesQuitThrough(t *testing.T) {
	if msg := scoped(3, tea.Quit)(); msg != tea.Quit() {
		t.Fatalf("quit was wrapped: %#v", msg)
	}
	if msg := scoped(3, func() tea.Msg { return "x" })(); msg != (scopedMsg{gen: 3, msg: "x"}) {
		t.Fatalf("got %#v", msg)
	}
}

func TestFooterOffersSwitching(t *testing.T) {
	opts, _ := testOptions(t)
	if m := ready(t, opts); !strings.Contains(m.View(), ui.HelpEntry("w", "workspace")) {
		t.Error("footer does not offer w with a sign-in")
	}
}
