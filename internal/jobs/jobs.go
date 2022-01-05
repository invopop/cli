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
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
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
		RunE: func(cmd *cobra.Command, _ []string) error {
			color.HiRed("choose a job operation")
			return cmd.Usage()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "send [workflow ID] [file.json]",
		Short: "Send a GOBL envelope for processing",
		Args:  cobra.ExactArgs(2),
		RunE:  c.send,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "download [job ID]",
		Short: "Fetch a job's details and download the results",
		Args:  cobra.ExactArgs(1),
		RunE:  c.download,
	})
	return cmd
}

func (c *Command) send(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	j := new(invopop.Job)
	j.ID = uuid.NewV1().String()
	j.WorkflowID = args[0]

	color.HiYellow("Job ID: %s", j.ID)

	data, err := ioutil.ReadFile(args[1])
	if err != nil {
		return fmt.Errorf("reading file data: %w", err)
	}
	if len(data) == 0 {
		return errors.New("file is empty")
	}
	j.Envelope = data

	if err := c.client.Transform.CreateJob(ctx, j, invopop.WithWait(60)); err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	complete, err := j.Status()
	switch {
	case complete && err == nil:
		color.HiGreen("Job created and completed successfully")
	case complete && err != nil:
		color.HiRed("Job created, but failed: %s", err)
	default:
		color.HiGreen("Job created, completion is still pending")
	}

	d, _ := json.MarshalIndent(j, "", "    ")
	fmt.Println(string(d))

	if err := c.downloadAttachments(ctx, j); err != nil {
		return fmt.Errorf("failed to download attachments: %w", err)
	}
	return nil
}

func (c *Command) download(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	j := new(invopop.Job)
	j.ID = args[0]

	if err := c.client.Transform.FetchJob(ctx, j); err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}

	d, _ := json.MarshalIndent(j, "", "    ")
	fmt.Println(string(d))

	if err := c.storeEnvelope(ctx, j); err != nil {
		return fmt.Errorf("failed to store envelope: %w", err)
	}
	if err := c.downloadAttachments(ctx, j); err != nil {
		return fmt.Errorf("failed to download attachments: %w", err)
	}
	return nil
}

func (c *Command) storeEnvelope(ctx context.Context, j *invopop.Job) error {
	if len(j.Envelope) == 0 {
		return nil
	}

	fp := filepath.Join(".", "data", "output", j.ID, "envelope.json")
	if err := ioutil.WriteFile(fp, j.Envelope, 0644); err != nil {
		return fmt.Errorf("writing data: %w", err)
	}

	return nil
}

func (c *Command) downloadAttachments(ctx context.Context, j *invopop.Job) error {
	for _, a := range j.Attachments {
		np := filepath.Join(".", "data", "output", j.ID, a.ID)
		if err := os.MkdirAll(np, os.ModePerm); err != nil {
			return fmt.Errorf("preparing directory `%s`: %w", np, err)
		}

		fp := filepath.Join(np, a.Name)
		if err := c.downloadURL(ctx, a.URL, fp); err != nil {
			return err
		}

		color.HiGreen("downloaded file: %s", fp)
	}
	return nil
}

func (c *Command) downloadURL(ctx context.Context, url, path string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}
