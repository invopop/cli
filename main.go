package main

import (
	"fmt"
	"os"

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
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
	conf, err := config.New(configPaths...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

	return root.Execute(ctx)
}
