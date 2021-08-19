package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/ghodss/yaml"
	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/config"
	"github.com/invopop/invocli/pkg/sequence"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

const (
	serviceName = "invocli"
	configPath  = "config/config.yaml"
)

func main() {
	conf := config.New(configPath)
	seq := sequence.New(conf.API.Host, conf.API.Token)
	root := &cobra.Command{Use: serviceName}
	root.AddCommand(&cobra.Command{
		Use:   "build [file.yaml]",
		Short: "Convert the file into a GoBL json document",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return build(conf, args[0], seq)
		},
	})
	root.AddCommand(sequenceCommands(conf, seq))

	if err := root.Execute(); err != nil {
		panic(err.Error())
	}
}

func sequenceCommands(conf *config.Config, seq sequence.Client) *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			listSeries(seq)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Follow steps to create a new sequence",
		RunE: func(cmd *cobra.Command, args []string) error {
			createSeries(seq)
			return nil
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "next [series ID]",
		Short: "Create a new entry in the series",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nextSeriesEntry(seq, args[0])
			return nil
		},
	})

	return cmd
}

func build(conf *config.Config, file string, seq sequence.Client) error {
	// Read in the file
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	// Parse as a GoBL document and extract invoice
	env := new(gobl.Envelope)
	if err := yaml.Unmarshal(data, env); err != nil {
		return fmt.Errorf("parsing source YAML: %w", err)
	}
	inv := new(bill.Invoice)
	if err := env.Extract(inv); err != nil {
		return fmt.Errorf("extracting invoice: %w", err)
	}

	// Merge our configuration details
	if inv.UUID.IsZero() {
		inv.UUID = uuid.NewV1()
	}
	fmt.Printf("UUID IS: %v\n", inv.UUID.String())
	if inv.Code == "" {
		if err = requestInvoiceSeriesEntry(seq, inv, conf.SeriesID); err != nil {
			return fmt.Errorf("requesting series entry: %w", err)
		}
	}
	inv.Region = conf.Region
	inv.Supplier = conf.Supplier

	// Calculate and validate
	if err := inv.Calculate(); err != nil {
		return fmt.Errorf("calculating invoice totals: %w", err)
	}
	if err := inv.Validate(); err != nil {
		return fmt.Errorf("validating invoice: %w", err)
	}

	// Insert and Sign
	if err := env.Insert(inv); err != nil {
		return fmt.Errorf("inserting invoice into envelope: %w", err)
	}
	if err := env.Sign(conf.Key); err != nil {
		return fmt.Errorf("signing enveloper: %w", err)
	}

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

func requestInvoiceSeriesEntry(seq sequence.Client, inv *bill.Invoice, dSeriesID string) error {
	se := new(sequence.Entry)
	se.ID = inv.UUID.String()

	p := promptui.Prompt{
		Label:   "Series ID",
		Default: dSeriesID,
	}
	seriesID, err := p.Run()
	if err != nil {
		fmt.Printf("prompt failed: %v\n", err.Error())
		return err
	}

	if err := seq.CreateEntry(context.Background(), seriesID, se); err != nil {
		return err
	}

	inv.Code = se.Code
	return nil
}
