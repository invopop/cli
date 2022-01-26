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
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/cobrax"
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
		Use:   "job",
		Short: "Create and managed jobs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cobrax.Usage(cmd, "choose a job operation")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "send [workflow ID] [file.json]",
		Short: "Send a GOBL envelope for processing",
		Args:  cobrax.ExactArgs(2),
		RunE:  c.send,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "download [job ID]",
		Short: "Fetch a job's details and download the results",
		Args:  cobrax.ExactArgs(1),
		RunE:  c.download,
	})
	return cmd
}

func (c *command) send(cmd *cobra.Command, args []string) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(time.Second*61))
	defer cancel()
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

func (c *command) download(cmd *cobra.Command, args []string) error {
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

func (c *command) storeEnvelope(_ context.Context, j *invopop.Job) error {
	if len(j.Envelope) == 0 {
		return nil
	}

	fp := filepath.Join(".", "data", "output", j.ID, "envelope.json")
	if err := ioutil.WriteFile(fp, j.Envelope, 0644); err != nil {
		return fmt.Errorf("writing data: %w", err)
	}

	return nil
}

func (c *command) downloadAttachments(ctx context.Context, j *invopop.Job) error {
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

func (c *command) downloadURL(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close() // nolint:errcheck

	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close() // nolint:errcheck

	_, err = io.Copy(out, resp.Body)
	return err
}
