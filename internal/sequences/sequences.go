package sequences

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/manifoldco/promptui"
	"github.com/rodaine/table"
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/warn"
)

type Command struct {
	client *invopop.Client
}

func New(ic *invopop.Client) *Command {
	c := new(Command)
	c.client = ic
	return c
}

func (c *Command) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "series",
		Short: "Manage sequence series",
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("choose a series operation")
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "Show a list of sequence series that belong to the client",
		RunE:  c.list,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Follow steps to create a new sequence",
		RunE:  c.create,
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "next [series ID]",
		Short: "Create a new entry in the series",
		Args:  cobra.ExactArgs(1),
		RunE:  c.next,
	})

	return cmd
}

func (c *Command) list(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	sc := new(invopop.SeriesCollection)
	if err := c.client.Sequence.ListSeries(ctx, sc); err != nil {
		return err
	}

	if len(sc.List) == 0 {
		return warn.Warn("No series found")
	}

	tbl := table.New("ID", "Name", "Prefix", "Padding", "Suffix", "Last Idx.", "Created At")
	tbl.
		WithHeaderFormatter(color.New(color.FgGreen, color.Underline).SprintfFunc()).
		WithFirstColumnFormatter(color.New(color.FgYellow).SprintfFunc())
	for _, s := range sc.List {
		tbl.AddRow(s.ID, s.Name, s.Prefix, s.Padding, s.Suffix, s.LastIndex, s.CreatedAt)
	}
	tbl.Print()
	return nil
}

func (c *Command) create(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	var err error
	s := new(invopop.Series)
	s.ID = uuid.NewV4().String()

	p := promptui.Prompt{
		Label: "Series Name",
	}
	s.Name, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Series Prefix",
	}
	s.Prefix, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	p = promptui.Prompt{
		Label: "Series Suffix",
	}
	s.Suffix, err = p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	ps := promptui.Select{
		Label: "Padding",
		Items: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"},
	}
	_, si, err := ps.Run()
	if err != nil {
		return fmt.Errorf("prompt select failed: %w", err)
	}
	i, _ := strconv.ParseInt(si, 10, 32)
	s.Padding = int32(i)
	p = promptui.Prompt{
		Label: "Start number",
		Validate: func(in string) error {
			if in == "" {
				return nil
			}
			_, err := strconv.ParseInt(in, 10, 32)
			return err
		},
	}
	sn, err := p.Run()
	if err != nil {
		return fmt.Errorf("prompt failed: %w", err)
	}
	if sn != "" {
		si, _ := strconv.ParseInt(sn, 10, 32)
		s.Start = int32(si)
	}

	if err := c.client.Sequence.CreateSeries(ctx, s); err != nil {
		return err
	}

	d, _ := json.MarshalIndent(s, "", "    ")
	fmt.Println(string(d))
	return nil
}

func (c *Command) next(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	seriesID := args[0]

	se := new(invopop.SeriesEntry)
	se.ID = uuid.NewV1().String()
	if err := c.client.Sequence.CreateEntry(ctx, seriesID, se); err != nil {
		return err
	}

	d, _ := json.MarshalIndent(se, "", "    ")
	fmt.Println(string(d))
	return nil
}
