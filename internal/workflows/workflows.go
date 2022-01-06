package workflows

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

// command keeps together the functionality required for jobs.
type command struct {
	client *invopop.Client
}

// New returns a new command instance
func New(ic *invopop.Client) *cobra.Command {
	c := new(command)
	c.client = ic

	cmd := &cobra.Command{
		Use:   "workflow",
		Short: "Create and managed workflows",
		RunE: func(cmd *cobra.Command, _ []string) error {
			color.HiRed("choose a workflow operation")
			return cmd.Usage()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Produce a table of all the current workflows.",
		RunE:  c.list,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new workflow step by step.",
		RunE:  c.create,
	})

	return cmd
}

func (c *command) list(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	wc := new(invopop.WorkflowCollection)
	if err := c.client.Transform.ListWorkflows(ctx, wc); err != nil {
		return err
	}

	if len(wc.List) == 0 {
		return warn.Warn("No workflows found")
	}

	tbl := table.New("ID", "Name", "Desc", "Disabled", "Task IDs", "Created At")
	tbl.
		WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
		WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())
	for _, w := range wc.List {
		tbl.AddRow(w.ID, w.Name, w.Description, w.Disabled, w.TaskIDs, w.CreatedAt)
	}
	tbl.Print()
	return nil
}

func (c *command) create(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var err error
	w := new(invopop.Workflow)
	w.ID = uuid.NewV4().String()

	p := promptui.Prompt{
		Label: "Workflow Name",
	}
	w.Name, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Workflow Description",
	}
	w.Description, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	// Fetch a list of tasks
	tc := new(invopop.TaskCollection)
	if err := c.client.Transform.ListTasks(ctx, tc); err != nil {
		return fmt.Errorf("index failed: %w", err)
	}
	ary := make([]string, 0)
	w.TaskIDs, err = c.taskLoop(ary, tc)
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if err := c.client.Transform.CreateWorkflow(ctx, w); err != nil {
		return fmt.Errorf("creating workflow failed: %w", err)
	}

	color.HiGreen("Workflow created successfully")
	d, _ := json.MarshalIndent(w, "", "    ")
	fmt.Println(string(d))
	return nil
}

// taskLoop helps ask for an array of tasks
func (c *command) taskLoop(ary []string, tc *invopop.TaskCollection) ([]string, error) {
	list := make([]string, len(tc.List))
	for i, t := range tc.List {
		list[i] = fmt.Sprintf("%s- %s", t.ID, t.Name)
	}
	list = append(list, "none")

	ps := promptui.Select{
		Label: fmt.Sprintf("Add Task %d", len(ary)+1),
		Items: list,
	}
	i, _, err := ps.Run()
	if err != nil {
		return nil, err
	}
	if i == (len(list) - 1) {
		return ary, nil
	}
	t := tc.List[i]
	ary = append(ary, t.ID)

	// Remove the selected entry
	tc.List = append(tc.List[:i], tc.List[i+1:]...)

	return c.taskLoop(ary, tc)
}
