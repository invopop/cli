package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/ghodss/yaml"
	"github.com/invopop/gobl"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/internal/config"
	"github.com/spf13/cobra"
)

const (
	serviceName = "invocli"
	configPath  = "config/config.yaml"
)

func main() {
	conf := config.New(configPath)

	cmdBuild := &cobra.Command{
		Use:   "build [file.yaml]",
		Short: "Convert the file into a GoBL json document",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return build(conf, args[0])
		},
	}

	rootCmd := &cobra.Command{Use: serviceName}
	rootCmd.AddCommand(cmdBuild)
	if err := rootCmd.Execute(); err != nil {
		panic(err.Error())
	}
}

func build(conf *config.Config, file string) error {
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
	inv.Code = "1234"
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
