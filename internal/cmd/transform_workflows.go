package cmd

import (
	"encoding/json"

	iohelper "github.com/invopop/cli/internal/io"
	"github.com/invopop/client.go/invopop"
	"github.com/spf13/cobra"
)

func transformWorkflowsCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	wf := &cobra.Command{
		Use:   "workflows",
		Short: "Manage transform workflows",
	}

	wf.AddCommand(transformWorkflowsListCmd(getClient))
	wf.AddCommand(transformWorkflowsCreateCmd(getClient))
	wf.AddCommand(transformWorkflowsFetchCmd(getClient))
	wf.AddCommand(transformWorkflowsUpdateCmd(getClient))

	return wf
}

func transformWorkflowsListCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		limit  int32
		schema string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workflows",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			col, err := c.Transform().Workflows().List(cmd.Context(), &invopop.FindWorkflows{
				Limit:  limit,
				Schema: schema,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(col)
		},
	}
	cmd.Flags().Int32Var(&limit, "limit", 0, "Maximum number of results")
	cmd.Flags().StringVar(&schema, "schema", "", "Filter by schema")
	return cmd
}

func transformWorkflowsCreateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workflow",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := iohelper.ReadInput(file)
			if err != nil {
				return err
			}
			var req invopop.CreateWorkflow
			if err := json.Unmarshal(data, &req); err != nil {
				return err
			}
			c, err := getClient()
			if err != nil {
				return err
			}
			wf, err := c.Transform().Workflows().Create(cmd.Context(), &req)
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(wf)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to workflow definition (JSON or YAML)")
	return cmd
}

func transformWorkflowsFetchCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <id>",
		Short: "Fetch a workflow by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			wf, err := c.Transform().Workflows().Fetch(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(wf)
		},
	}
}

func transformWorkflowsUpdateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "update <id>",
		Short: "Update a workflow",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := iohelper.ReadInput(file)
			if err != nil {
				return err
			}
			var req invopop.UpdateWorkflow
			if err := json.Unmarshal(data, &req); err != nil {
				return err
			}
			req.ID = args[0]
			c, err := getClient()
			if err != nil {
				return err
			}
			wf, err := c.Transform().Workflows().Update(cmd.Context(), &req)
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(wf)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to workflow definition (JSON or YAML)")
	return cmd
}

func transformCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	transform := &cobra.Command{
		Use:   "transform",
		Short: "Manage transform jobs and workflows",
	}

	transform.AddCommand(transformJobsCmd(getClient))
	transform.AddCommand(transformWorkflowsCmd(getClient))

	return transform
}
