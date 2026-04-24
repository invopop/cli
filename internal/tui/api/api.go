// Package api provides tea.Cmd factories and message types that wrap the
// Invopop client so the TUI can perform asynchronous API calls.
package api

import (
	"context"
	"encoding/json"
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/invopop/client.go/invopop"
)

// WorkspaceFetchedMsg is emitted once the active workspace is loaded.
type WorkspaceFetchedMsg struct {
	Workspace *invopop.Workspace
}

// EntriesFetchedMsg carries a page of silo entries for a given folder.
type EntriesFetchedMsg struct {
	Collection *invopop.SiloEntryCollection
	Folder     string
}

// EntryFetchedMsg carries a single silo entry that was just loaded.
type EntryFetchedMsg struct {
	Entry *invopop.SiloEntry
}

// EntryUpdatedMsg carries the result of a silo entry update.
type EntryUpdatedMsg struct {
	Entry *invopop.SiloEntry
}

// GOBLBuiltMsg carries the built GOBL document payload.
type GOBLBuiltMsg struct {
	Data json.RawMessage
}

// SearchResultCollection mirrors the shape of the upcoming silo search
// endpoint response so the TUI can keep its wiring while the API rolls out.
type SearchResultCollection struct {
	List []*invopop.SiloEntry
}

// SearchResultsMsg carries the results of a silo entry search.
type SearchResultsMsg struct {
	Collection *SearchResultCollection
}

// WorkflowsFetchedMsg carries a page of workflows.
type WorkflowsFetchedMsg struct {
	Collection *invopop.WorkflowCollection
}

// WorkflowFetchedMsg carries a single workflow.
type WorkflowFetchedMsg struct {
	Workflow *invopop.Workflow
}

// WorkflowUpdatedMsg carries the result of a workflow update.
type WorkflowUpdatedMsg struct {
	Workflow *invopop.Workflow
}

// JobFetchedMsg carries a single transform job.
type JobFetchedMsg struct {
	Job *invopop.Job
}

// ErrorMsg is emitted by any API command that fails. Context describes
// the operation that was being attempted when the error occurred.
type ErrorMsg struct {
	Err     error
	Context string
}

// Error returns a human-readable error message, prefixed with the context
// when available.
func (e ErrorMsg) Error() string {
	if e.Context != "" {
		return e.Context + ": " + e.Err.Error()
	}
	return e.Err.Error()
}

// FetchWorkspace returns a tea.Cmd that loads the active workspace.
func FetchWorkspace(client *invopop.Client) tea.Cmd {
	return func() tea.Msg {
		ws, err := client.Access().Workspace().Fetch(context.Background())
		if err != nil {
			return ErrorMsg{Err: err, Context: "fetch workspace"}
		}
		return WorkspaceFetchedMsg{Workspace: ws}
	}
}

// FetchEntries returns a tea.Cmd that lists silo entries for the given folder.
func FetchEntries(client *invopop.Client, folder string, cursor string, limit int32) tea.Cmd {
	return func() tea.Msg {
		req := &invopop.FindSiloEntries{
			Folder: folder,
			Cursor: cursor,
			Limit:  limit,
		}
		col, err := client.Silo().Entries().List(context.Background(), req)
		if err != nil {
			return ErrorMsg{Err: err, Context: "fetch entries"}
		}
		return EntriesFetchedMsg{Collection: col, Folder: folder}
	}
}

// FetchEntry returns a tea.Cmd that loads a single silo entry by ID.
func FetchEntry(client *invopop.Client, id string) tea.Cmd {
	return func() tea.Msg {
		entry, err := client.Silo().Entries().Fetch(context.Background(), id)
		if err != nil {
			return ErrorMsg{Err: err, Context: "fetch entry"}
		}
		return EntryFetchedMsg{Entry: entry}
	}
}

// UpdateEntry returns a tea.Cmd that updates a silo entry.
func UpdateEntry(client *invopop.Client, req *invopop.UpdateSiloEntry) tea.Cmd {
	return func() tea.Msg {
		entry, err := client.Silo().Entries().Update(context.Background(), req)
		if err != nil {
			return ErrorMsg{Err: err, Context: "update entry"}
		}
		return EntryUpdatedMsg{Entry: entry}
	}
}

// BuildGOBL returns a tea.Cmd that builds a GOBL document from the provided payload.
func BuildGOBL(client *invopop.Client, data json.RawMessage) tea.Cmd {
	return func() tea.Msg {
		result, err := client.Silo().GOBL().Build(context.Background(), &invopop.BuildGOBL{
			Data: data,
		})
		if err != nil {
			return ErrorMsg{Err: err, Context: "build GOBL"}
		}
		return GOBLBuiltMsg{Data: result.Data}
	}
}

// SearchEntries is a placeholder until the public `GET /silo/v1/search`
// endpoint and client.go binding are available. It keeps the TUI wiring intact
// but surfaces a clear error when the user invokes search.
func SearchEntries(_ *invopop.Client, _, _ string, _, _ int32) tea.Cmd {
	return func() tea.Msg {
		return ErrorMsg{
			Err:     errors.New("silo search endpoint is not yet available in the public API"),
			Context: "search entries",
		}
	}
}

// FetchWorkflows returns a tea.Cmd that lists workflows, optionally filtered by schema.
func FetchWorkflows(client *invopop.Client, schema string, limit int32) tea.Cmd {
	return func() tea.Msg {
		col, err := client.Transform().Workflows().List(context.Background(), &invopop.FindWorkflows{
			Schema: schema,
			Limit:  limit,
		})
		if err != nil {
			return ErrorMsg{Err: err, Context: "fetch workflows"}
		}
		return WorkflowsFetchedMsg{Collection: col}
	}
}

// FetchWorkflow returns a tea.Cmd that loads a single workflow by ID.
func FetchWorkflow(client *invopop.Client, id string) tea.Cmd {
	return func() tea.Msg {
		wf, err := client.Transform().Workflows().Fetch(context.Background(), id)
		if err != nil {
			return ErrorMsg{Err: err, Context: "fetch workflow"}
		}
		return WorkflowFetchedMsg{Workflow: wf}
	}
}

// UpdateWorkflow returns a tea.Cmd that updates a workflow.
func UpdateWorkflow(client *invopop.Client, req *invopop.UpdateWorkflow) tea.Cmd {
	return func() tea.Msg {
		wf, err := client.Transform().Workflows().Update(context.Background(), req)
		if err != nil {
			return ErrorMsg{Err: err, Context: "update workflow"}
		}
		return WorkflowUpdatedMsg{Workflow: wf}
	}
}

// FetchJob returns a tea.Cmd that loads a single transform job by ID.
func FetchJob(client *invopop.Client, id string) tea.Cmd {
	return func() tea.Msg {
		job, err := client.Transform().Jobs().Fetch(context.Background(), id)
		if err != nil {
			return ErrorMsg{Err: err, Context: "fetch job"}
		}
		return JobFetchedMsg{Job: job}
	}
}
