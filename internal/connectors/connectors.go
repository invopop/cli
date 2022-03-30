package connectors

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/cobrax"
	"github.com/invopop/invocli/internal/warn"
)

// command keeps together the functionality required for connectors.
type command struct {
	client *invopop.Client
}

// New returns a new command instance
func New(ic *invopop.Client) *cobra.Command {
	c := new(command)
	c.client = ic

	cmd := &cobra.Command{
		Use:   "connector",
		Short: "Create and manage connectors",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cobrax.Usage(cmd, "choose a connector operation")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Produce a table of all the current connectors.",
		RunE:  c.list,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new connector.",
		RunE:  c.create,
	})

	return cmd
}

func (c *command) list(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	col := new(invopop.ConnectorCollection)
	if err := c.client.Transform().ListConnectors(ctx, col); err != nil {
		return err
	}
	if len(col.List) == 0 {
		return warn.Warn("No connectors found")
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

func (c *command) create(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var err error
	t := new(invopop.Connector)
	t.ID = uuid.NewV4().String()

	p := promptui.Prompt{
		Label: "Connector Name",
	}
	t.Name, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Notes",
	}
	t.Notes, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	ps := promptui.SelectWithAdd{
		Label:    "Provider",
		Items:    []string{"core-tag", "core-markdown", "es-bill-pdf"},
		AddLabel: "Other",
	}
	_, t.Provider, err = ps.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Connector Configuration (JSON)",
	}
	data, err := p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	if len(data) > 0 {
		t.Config = json.RawMessage(data)
	}

	if err := c.client.Transform().CreateConnector(ctx, t); err != nil {
		return fmt.Errorf("creating connector failed: %w", err)
	}

	color.HiGreen("Connector created successfully")
	d, _ := json.MarshalIndent(t, "", "    ")
	fmt.Println(string(d))
	return nil
}
