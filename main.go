package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/invopop/client.go/invopop"
	"github.com/invopop/invocli/internal/builder"
	"github.com/invopop/invocli/internal/config"
	"github.com/invopop/invocli/internal/jobs"
	"github.com/invopop/invocli/internal/sequences"
	"github.com/invopop/invocli/internal/tasks"
	"github.com/invopop/invocli/internal/workflows"
)

const serviceName = "invocli"

var configPaths = []string{"~/.invocli/config.yaml", "/etc/invocli/config.yaml", "./config/config.yaml", "config.yaml"}

func main() {
	if err := run(); err != nil {
		var werr interface {
			Warn()
		}
		if errors.As(err, &werr) {
			color.HiYellow(err.Error())
		} else {
			color.HiRed(err.Error())
			os.Exit(1)
		}
	}
	os.Exit(0)
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	conf, err := config.New(configPaths...)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ic := invopop.New(conf.API.Host, conf.API.Token)

	seqs := sequences.New(ic)
	wkfl := workflows.New(ic)

	root := &cobra.Command{
		Use:          serviceName,
		SilenceUsage: true,
	}
	root.AddCommand(builder.New(ic, conf))
	root.AddCommand(seqs.Command())
	root.AddCommand(jobs.New(ic))
	root.AddCommand(tasks.New(ic))
	root.AddCommand(wkfl.Command())

	return root.ExecuteContext(ctx)
}
