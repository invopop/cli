package cmd

import (
	"fmt"

	"github.com/invopop/client.go/invopop"
	iohelper "github.com/invopop/invocli/internal/io"
	"github.com/spf13/cobra"
)

func transformJobsCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	jobs := &cobra.Command{
		Use:   "jobs",
		Short: "Manage transform jobs",
	}

	jobs.AddCommand(transformJobsListCmd(getClient))
	jobs.AddCommand(transformJobsCreateCmd(getClient))
	jobs.AddCommand(transformJobsFetchCmd(getClient))
	jobs.AddCommand(transformJobsFetchKeyCmd(getClient))

	return jobs
}

func transformJobsListCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		createdAt string
		limit     int32
		cursor    string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List transform jobs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			col, err := c.Transform().Jobs().List(cmd.Context(), &invopop.FindJobs{
				CreatedAt: createdAt,
				Limit:     limit,
				Cursor:    cursor,
			})
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(col)
		},
	}
	cmd.Flags().StringVar(&createdAt, "created-at", "", "Date from which results are returned in descending order (RFC3339)")
	cmd.Flags().Int32Var(&limit, "limit", 0, "Maximum number of results (up to 100)")
	cmd.Flags().StringVar(&cursor, "cursor", "", "Pagination cursor")
	return cmd
}

func transformJobsCreateCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	var (
		workflowID  string
		siloEntryID string
		file        string
		key         string
		wait        int32
		tags        []string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a transform job",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if workflowID == "" {
				return fmt.Errorf("--workflow-id is required")
			}
			if siloEntryID != "" && file != "" {
				return fmt.Errorf("--silo-entry-id and --file are mutually exclusive")
			}

			req := &invopop.CreateJob{
				WorkflowID:  workflowID,
				SiloEntryID: siloEntryID,
				Key:         key,
				Tags:        tags,
				Wait:        wait,
			}

			// If no silo entry ID, read data from file/stdin
			if siloEntryID == "" {
				data, err := iohelper.ReadInput(file)
				if err != nil {
					return err
				}
				req.Data = data
			}

			c, err := getClient()
			if err != nil {
				return err
			}
			job, err := c.Transform().Jobs().Create(cmd.Context(), req)
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(job)
		},
	}
	cmd.Flags().StringVar(&workflowID, "workflow-id", "", "Workflow ID (required)")
	cmd.Flags().StringVar(&siloEntryID, "silo-entry-id", "", "Silo entry ID (alternative to --file/stdin)")
	cmd.Flags().StringVarP(&file, "file", "f", "", "Path to GOBL document (JSON or YAML)")
	cmd.Flags().StringVar(&key, "key", "", "Idempotency key")
	cmd.Flags().Int32Var(&wait, "wait", 0, "Seconds to wait for job completion")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "Tags for the job")
	return cmd
}

func transformJobsFetchCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch <id>",
		Short: "Fetch a job by ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			job, err := c.Transform().Jobs().Fetch(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(job)
		},
	}
}

func transformJobsFetchKeyCmd(getClient func() (*invopop.Client, error)) *cobra.Command {
	return &cobra.Command{
		Use:   "fetch-key <key>",
		Short: "Fetch a job by key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := getClient()
			if err != nil {
				return err
			}
			job, err := c.Transform().Jobs().FetchByKey(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return iohelper.PrintJSON(job)
		},
	}
}
