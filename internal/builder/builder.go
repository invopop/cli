package builder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/ghodss/yaml"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/note"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/cobrax"
	"github.com/invopop/invocli/internal/config"
)

// command keeps together the functionality required to build
// GOBL documents.
type command struct {
	client *invopop.Client
	conf   *config.Config // used for defaults
}

// New prepares a new builder command
func New(ic *invopop.Client, conf *config.Config) *cobra.Command {
	c := new(command)
	c.client = ic
	c.conf = conf

	return &cobra.Command{
		Use:   "build [file.yaml]",
		Short: "Convert the file into a GoBL json document",
		Args:  cobrax.MinimumNArgs(1),
		RunE:  c.build,
	}
}

func (c *command) build(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	file := args[0]

	env, err := c.buildEnvelope(ctx, file)
	if err != nil {
		return err
	}
	if err := c.writeFile(env, file); err != nil {
		return fmt.Errorf("unable to write file output: %w", err)
	}
	return nil
}

func (c *command) buildEnvelope(ctx context.Context, file string) (*gobl.Envelope, error) {
	// Read in the file
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	// Parse as a GoBL document
	env := gobl.NewEnvelope()
	if err := yaml.Unmarshal(data, env); err != nil {
		return nil, fmt.Errorf("parsing source YAML: %w", err)
	}
	if env.Head == nil {
		return nil, errors.New("invalid/empty header")
	}
	if env.Document == nil {
		return nil, errors.New("invalid/empty document")
	}
	if env.Head.UUID.IsZero() {
		env.Head.UUID = uuid.NewV1()
	}

	switch env.Document.Schema.Interface().(type) {
	case *bill.Invoice:
		if err := c.buildInvoice(ctx, env); err != nil {
			return nil, err
		}
	case *note.Message:
		if err := c.buildMessage(env); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unrecognized envelope schema: %v", env.Document.Schema)
	}

	if err := env.Sign(c.conf.Key); err != nil {
		return nil, fmt.Errorf("signing envelope: %w", err)
	}
	if err := env.Validate(); err != nil {
		return nil, fmt.Errorf("invalid envelope: %w", err)
	}
	return env, nil
}

func (c *command) buildInvoice(ctx context.Context, env *gobl.Envelope) error {
	inv := new(bill.Invoice)

	if err := env.Extract(inv); err != nil {
		return fmt.Errorf("extracting invoice: %w", err)
	}

	// Merge our configuration details to complete the invoice details
	if inv.Code == "" {
		if inv.UUID == nil {
			id := uuid.NewV4()
			inv.UUID = &id
		}
		if err := c.requestInvoiceSeriesEntry(ctx, inv); err != nil {
			return fmt.Errorf("requesting series entry: %w", err)
		}
	}
	if inv.Supplier == nil {
		inv.Supplier = c.conf.Supplier
	}

	// re-insert
	if err := env.Insert(inv); err != nil {
		return fmt.Errorf("inserting invoice into envelope: %w", err)
	}

	return nil
}

// buildMesage doesn't need to do much, just validates the basic data
// and outputs a JSON.
func (c *command) buildMessage(env *gobl.Envelope) error {
	return env.Complete()
}

func (c *command) writeFile(env *gobl.Envelope, file string) error {
	// Output to file
	out, err := json.MarshalIndent(env, "", "	")
	if err != nil {
		return fmt.Errorf("marshalling output: %w", err)
	}
	ext := path.Ext(file)
	outfile := file[0:len(file)-len(ext)] + ".json"
	if err := ioutil.WriteFile(outfile, out, 0644); err != nil {
		return fmt.Errorf("writing json output: %w", err)
	}

	fmt.Printf("document created successfully: %v\n", outfile)
	return nil
}

func (c *command) requestInvoiceSeriesEntry(ctx context.Context, inv *bill.Invoice) error {
	if inv.UUID == nil {
		return errors.New("cannot request sequence entry without Invoice UUID")
	}
	se := new(invopop.SeriesEntry)
	se.ID = inv.UUID.String()

	p := promptui.Prompt{
		Label:   "Series ID",
		Default: c.conf.SeriesID,
	}
	seriesID, err := p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}

	if err := c.client.Sequence.CreateEntry(ctx, seriesID, se); err != nil {
		return err
	}

	inv.Code = se.Code
	return nil
}
