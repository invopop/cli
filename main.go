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
	"github.com/invopop/invocli/internal/connectors"
	"github.com/invopop/invocli/internal/jobs"
	"github.com/invopop/invocli/internal/keys"
	"github.com/invopop/invocli/internal/sequences"
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
			return
		}

		color.HiRed(err.Error())
		var usageErr interface {
			UsageCmd() *cobra.Command
		}
		if errors.As(err, &usageErr) {
			if err := usageErr.UsageCmd().Usage(); err != nil {
				color.HiRed(err.Error())
			}
		}
		os.Exit(1)
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

	root := &cobra.Command{
		Use:           serviceName,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(builder.New(ic, conf))
	root.AddCommand(sequences.New(ic))
	root.AddCommand(jobs.New(ic))
	root.AddCommand(connectors.New(ic))
	root.AddCommand(workflows.New(ic))
	root.AddCommand(keys.New())

	return root.ExecuteContext(ctx)
}
