package cobrax

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
)

func TestUsage(t *testing.T) {
	const (
		wantName = "short"
		wantMsg  = "foo"
	)
	cmd := &cobra.Command{Short: wantName}
	err := Usage(cmd, wantMsg)
	checkError(t, err, wantName, wantMsg)
}

func TestExactArgs(t *testing.T) {
	t.Run("correct args", func(t *testing.T) {
		cmd := &cobra.Command{}
		err := ExactArgs(1)(cmd, []string{"one"})
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		const wantName = "shortName"
		cmd := &cobra.Command{Short: wantName}
		err := ExactArgs(1)(cmd, []string{})
		checkError(t, err, wantName, "accepts 1 arg(s), received 0")
	})
}

func TestMinimumNArgs(t *testing.T) {
	t.Run("correct args", func(t *testing.T) {
		cmd := &cobra.Command{}
		err := MinimumNArgs(1)(cmd, []string{"one", "two"})
		if err != nil {
			t.Errorf("Unexpected error: %s", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		const wantName = "shortName"
		cmd := &cobra.Command{Short: wantName}
		err := MinimumNArgs(1)(cmd, []string{})
		checkError(t, err, wantName, "requires at least 1 arg(s), only received 0")
	})
}

// checkError asserts that:
//
// - (err == nil && msg == "") || (err.Error() == msg)
// - err is of type interface{ UsageCmd *cobra.Command}
// - that cmd.Short matches shortName
func checkError(t *testing.T, err error, shortName, msg string) {
	t.Helper()
	if msg == "" {
		if err == nil {
			return
		}
		t.Errorf("Got an error, expected none")
		return
	}
	if err.Error() != msg {
		t.Errorf("Unexpected error: %s", err)
	}
	var uerr interface {
		UsageCmd() *cobra.Command
	}
	if errors.As(err, &uerr) {
		if short := uerr.UsageCmd().Short; short != shortName {
			t.Errorf("Unexpected command returned: %s", short)
		}
		return
	}
	t.Errorf("Not a Usage error")
}
