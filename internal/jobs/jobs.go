package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
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
		Use:   "job",
		Short: "Create and managed jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("choose a job operation")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "send [workflow ID] [file.yaml]",
		Short: "Send a GOBL envelope for processing",
		Args:  cobra.ExactArgs(2),
		Run:   c.send,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "download [job ID]",
		Short: "Fetch a job's details and download the results",
		Args:  cobra.ExactArgs(1),
		Run:   c.download,
	})
	return cmd
}

func (c *Command) send(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	j := new(invopop.Job)
	j.ID = uuid.NewV1().String()
	j.WorkflowID = args[0]

	color.HiYellow("Job ID: %s", j.ID)

	data, err := ioutil.ReadFile(args[1])
	if err != nil {
		color.HiRed("reading file data: %s", err.Error())
		return
	}
	if len(data) == 0 {
		color.HiRed("file is empty")
		return
	}
	j.Envelope = data

	if err := c.client.Transform.CreateJob(ctx, j, invopop.WithWait(60)); err != nil {
		color.HiRed("failed to create job: %s", err.Error())
		return
	}

	if j.CompletedAt != "" {
		color.HiGreen("Job created and completed successfully")
	} else {
		color.HiGreen("Job created, completion is still pending")
	}

	d, _ := json.MarshalIndent(j, "", "    ")
	fmt.Println(string(d))

	if err := c.downloadAttachments(ctx, j); err != nil {
		color.HiRed("failed to download attachments: %s", err.Error())
	}
}

func (c *Command) download(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	j := new(invopop.Job)
	j.ID = args[0]

	if err := c.client.Transform.FetchJob(ctx, j); err != nil {
		color.HiRed("failed to fetch job: %s", err.Error())
		return
	}

	d, _ := json.MarshalIndent(j, "", "    ")
	fmt.Println(string(d))

	if err := c.downloadAttachments(ctx, j); err != nil {
		color.HiRed("failed to download attachments: %s", err.Error())
	}
}

func (c *Command) downloadAttachments(ctx context.Context, j *invopop.Job) error {
	for _, a := range j.Attachments {
		np := filepath.Join(".", "data", "output", j.ID, a.ID)
		if err := os.MkdirAll(np, os.ModePerm); err != nil {
			return fmt.Errorf("preparing directory `%s`: %w", np, err)
		}

		// Get the data
		resp, err := http.Get(a.URL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Create the file
		fp := filepath.Join(np, a.Name)
		out, err := os.Create(fp)
		if err != nil {
			return err
		}
		defer out.Close()

		// Write the body to file
		_, err = io.Copy(out, resp.Body)
		if err != nil {
			return err
		}

		color.HiGreen("downloaded file: %s", fp)
	}
	return nil
}
