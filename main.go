package main

import (
	"github.com/invopop/client.go/invopop"
	"github.com/invopop/invocli/internal/builder"
	"github.com/invopop/invocli/internal/config"
	"github.com/invopop/invocli/internal/jobs"
	"github.com/invopop/invocli/internal/sequences"
	"github.com/invopop/invocli/internal/tasks"
	"github.com/invopop/invocli/internal/workflows"

	"github.com/spf13/cobra"
)

const (
	serviceName = "invocli"
)

var configPaths = []string{"~/.invocli/config.yaml", "/etc/invocli/config.yaml", "./config/config.yaml", "config.yaml"}

func main() {
	conf, err := config.New(configPaths...)
	if err != nil {
		panic(err.Error())
	}

	ic := invopop.New(conf.API.Host, conf.API.Token)

	bldr := builder.New(ic, conf)
	seqs := sequences.New(ic)
	jobs := jobs.New(ic)
	tsks := tasks.New(ic)
	wkfl := workflows.New(ic)

	root := &cobra.Command{Use: serviceName}
	root.AddCommand(bldr.Command())
	root.AddCommand(seqs.Command())
	root.AddCommand(jobs.Command())
	root.AddCommand(tsks.Command())
	root.AddCommand(wkfl.Command())

	if err := root.Execute(); err != nil {
		panic(err.Error())
	}
}
