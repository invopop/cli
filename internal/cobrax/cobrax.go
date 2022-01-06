// Package cobrax provides some helpers we use with cobra
package cobrax

import (
	"errors"

	"github.com/spf13/cobra"
)

type showUsage struct {
	error
	cmd *cobra.Command
}

func (e *showUsage) UsageCmd() *cobra.Command {
	return e.cmd
}

// Usage returns an error that will display the command's usage summary.
func Usage(cmd *cobra.Command, msg string) error {
	return &showUsage{
		error: errors.New(msg),
		cmd:   cmd,
	}
}

// ExactArgs wraps cobra.ExactArgs, wrapping any error with an intent to
// display usage info.
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.ExactArgs(n)(cmd, args); err != nil {
			return &showUsage{
				error: err,
				cmd:   cmd,
			}
		}
		return nil
	}
}

// MinimumNArgs wraps cobra.MinimumNArgs, wrapping any error with an intent to
// display usage info.
func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.MinimumNArgs(n)(cmd, args); err != nil {
			return &showUsage{
				error: err,
				cmd:   cmd,
			}
		}
		return nil
	}
}
