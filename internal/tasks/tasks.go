package tasks

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/warn"
)

// Command keeps together the functionality required for tasks.
type Command struct {
	client *invopop.Client
}

// New returns a new command instance
func New(ic *invopop.Client) *Command {
	c := new(Command)
	c.client = ic
	return c
}

func (c *Command) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Create and manage tasks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			color.HiRed("choose a task operation")
			return cmd.Usage()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Produce a table of all the current tasks.",
		RunE:  c.list,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new task.",
		RunE:  c.create,
	})

	return cmd
}

func (c *Command) list(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	col := new(invopop.TaskCollection)
	if err := c.client.Transform.ListTasks(ctx, col); err != nil {
		return err
	}
	if len(col.List) == 0 {
		return warn.Warn("No tasks found")
	}

	tbl := table.New("ID", "Name", "Notes", "Disabled", "Config", "Created At")
	tbl.
		WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
		WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())
	for _, m := range col.List {
		tbl.AddRow(m.ID, m.Name, m.Notes, m.Disabled, string(m.Config), m.CreatedAt)
	}
	tbl.Print()
	return nil
}

func (c *Command) create(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var err error
	t := new(invopop.Task)
	t.ID = uuid.NewV4().String()

	p := promptui.Prompt{
		Label: "Task Name",
	}
	t.Name, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Task Notes",
	}
	t.Notes, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	ps := promptui.SelectWithAdd{
		Label:    "Task Provider",
		Items:    []string{"core-tag", "core-markdown", "es-bill-pdf"},
		AddLabel: "Other",
	}
	_, t.Provider, err = ps.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Task Configuration (JSON)",
	}
	data, err := p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	if len(data) > 0 {
		t.Config = json.RawMessage(data)
	}

	if err := c.client.Transform.CreateTask(ctx, t); err != nil {
		return fmt.Errorf("creating task failed: %w", err)
	}

	color.HiGreen("Task created successfully")
	d, _ := json.MarshalIndent(t, "", "    ")
	fmt.Println(string(d))
	return nil
}
