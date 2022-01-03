package workflows

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
	"github.com/manifoldco/promptui"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"
)

// Command keeps together the functionality required for jobs.
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
		Use:   "workflow",
		Short: "Create and managed workflows",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("choose a workflow operation")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Produce a table of all the current workflows.",
		Run:   c.list,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new workflow step by step.",
		Run:   c.create,
	})

	return cmd
}

func (c *Command) list(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()

	wc := new(invopop.WorkflowCollection)
	if err := c.client.Transform.ListWorkflows(ctx, wc); err != nil {
		color.HiRed(err.Error())
		return
	}

	if len(wc.List) == 0 {
		color.HiYellow("No workflows found")
		return
	}

	tbl := table.New("ID", "Name", "Desc", "Disabled", "Task IDs", "Created At")
	tbl.
		WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
		WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())
	for _, w := range wc.List {
		tbl.AddRow(w.ID, w.Name, w.Description, w.Disabled, w.TaskIDs, w.CreatedAt)
	}
	tbl.Print()
}

func (c *Command) create(cmd *cobra.Command, args []string) {
	ctx := cmd.Context()
	var err error
	w := new(invopop.Workflow)
	w.ID = uuid.NewV4().String()

	p := promptui.Prompt{
		Label: "Workflow Name",
	}
	w.Name, err = p.Run()
	if err != nil {
		color.HiRed("prompt failed: %v", err.Error())
		return
	}
	p = promptui.Prompt{
		Label: "Workflow Description",
	}
	w.Description, err = p.Run()
	if err != nil {
		color.HiRed("prompt failed: %v", err.Error())
		return
	}

	// Fetch a list of tasks
	tc := new(invopop.TaskCollection)
	if err := c.client.Transform.ListTasks(ctx, tc); err != nil {
		color.HiRed("index failed: %v", err.Error())
		return
	}
	ary := make([]string, 0)
	w.TaskIDs = c.taskLoop(ary, tc)

	if err := c.client.Transform.CreateWorkflow(ctx, w); err != nil {
		color.HiRed("creating workflow failed: %v", err.Error())
		return
	}

	color.HiGreen("Workflow created successfully")
	d, _ := json.MarshalIndent(w, "", "    ")
	fmt.Println(string(d))
}

// taskLoop helps ask for an array of tasks
func (c *Command) taskLoop(ary []string, tc *invopop.TaskCollection) []string {
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
		color.HiRed("prompt failed: %v", err.Error())
		return ary
	}
	if i == (len(list) - 1) {
		return ary
	}
	t := tc.List[i]
	ary = append(ary, t.ID)

	// Remove the selected entry
	tc.List = append(tc.List[:i], tc.List[i+1:]...)

	return c.taskLoop(ary, tc)
}
