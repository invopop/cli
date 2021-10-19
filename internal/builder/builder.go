package builder

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/note"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/config"

	"github.com/fatih/color"
	"github.com/ghodss/yaml"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// Command keeps together the functionality required to build
// GOBL documents.
type Command struct {
	client *invopop.Client
	conf   *config.Config // used for defaults
}

// New prepares a new builder command
func New(ic *invopop.Client, conf *config.Config) *Command {
	c := new(Command)
	c.client = ic
	c.conf = conf
	return c
}

func (c *Command) Command() *cobra.Command {
	return &cobra.Command{
		Use:   "build [file.yaml]",
		Short: "Convert the file into a GoBL json document",
		Args:  cobra.MinimumNArgs(1),
		Run:   c.build,
	}
}

func (c *Command) build(cmd *cobra.Command, args []string) {
	ctx := context.Background()
	file := args[0]

	// Read in the file
	data, err := ioutil.ReadFile(file)
	if err != nil {
		color.HiRed("reading file data: %s", err.Error())
		return
	}

	// Parse as a GoBL document
	env := new(gobl.Envelope)
	if err := yaml.Unmarshal(data, env); err != nil {
		color.HiRed("parsing source YAML error: %s", err.Error())
		return
	}

	switch env.Head.Type {
	case bill.InvoiceType:
		if err := c.buildInvoice(ctx, env); err != nil {
			color.HiRed(err.Error())
			return
		}
	case note.MessageType:
		if err := c.buildMessage(env); err != nil {
			color.HiRed(err.Error())
			return
		}
	default:
		color.HiRed(`unrecognised envelope type: "%v"`, env.Head.Type)
		return
	}

	if err := env.Sign(c.conf.Key); err != nil {
		color.HiRed("signing envelope: %s", err.Error())
		return
	}
	if err := env.Validate(); err != nil {
		color.HiRed("invalid envelope: %s", err.Error())
		return
	}

	if err := c.writeFile(env, file); err != nil {
		color.HiRed("unable to write file output: %v", err.Error())
	}
}

func (c *Command) buildInvoice(ctx context.Context, env *gobl.Envelope) error {
	inv := new(bill.Invoice)
	if err := env.Extract(inv); err != nil {
		return fmt.Errorf("extracting invoice: %w", err)
	}

	// Merge our configuration details to complete the invoice details
	if inv.UUID.IsZero() {
		inv.UUID = uuid.NewV1()
	}
	fmt.Printf("UUID IS: %v\n", inv.UUID.String())
	if inv.Code == "" {
		if err := c.requestInvoiceSeriesEntry(ctx, inv); err != nil {
			return fmt.Errorf("requesting series entry: %w", err)
		}
	}
	inv.Region = c.conf.Region
	inv.Supplier = c.conf.Supplier

	// Calculate and validate
	if err := inv.Calculate(); err != nil {
		return fmt.Errorf("calculating invoice totals: %w", err)
	}
	if err := inv.Validate(); err != nil {
		return fmt.Errorf("validating invoice: %w", err)
	}

	// re-insert
	if err := env.Insert(inv); err != nil {
		return fmt.Errorf("inserting invoice into envelope: %w", err)
	}

	return nil
}

// buildMesage doesn't need to do much, just validates the basic data
// and outputs a JSON.
func (c *Command) buildMessage(env *gobl.Envelope) error {
	msg := new(note.Message)
	if err := env.Extract(msg); err != nil {
		return fmt.Errorf("extracting message: %w", err)
	}

	// Assign UUID if not there already
	if env.Head.UUID.IsZero() {
		env.Head.UUID = uuid.NewV1()
	}

	if err := msg.Validate(); err != nil {
		return fmt.Errorf("validating message: %w", err)
	}

	if err := env.Insert(msg); err != nil {
		return fmt.Errorf("inserting message into envelope: %w", err)
	}

	return nil
}

func (c *Command) writeFile(env *gobl.Envelope, file string) error {
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

	fmt.Printf("invoice created successfully: %v\n", outfile)
	return nil
}

func (c *Command) requestInvoiceSeriesEntry(ctx context.Context, inv *bill.Invoice) error {
	se := new(invopop.SeriesEntry)
	se.ID = inv.UUID.String()

	p := promptui.Prompt{
		Label:   "Series ID",
		Default: c.conf.SeriesID,
	}
	seriesID, err := p.Run()
	if err != nil {
		fmt.Printf("prompt failed: %v\n", err.Error())
		return err
	}

	if err := c.client.Sequence.CreateEntry(ctx, seriesID, se); err != nil {
		return err
	}

	inv.Code = se.Code
	return nil
}
