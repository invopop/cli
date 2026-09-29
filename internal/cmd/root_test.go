package cmd

import (
	"strings"
	"testing"
)

func TestFlagOrEnv(t *testing.T) {
	const env = "INVOCLI_TEST_VALUE"

	t.Run("flag takes precedence", func(t *testing.T) {
		t.Setenv(env, "from-env")
		if got := flagOrEnv("from-flag", env); got != "from-flag" {
			t.Errorf("got %q, want %q", got, "from-flag")
		}
	})

	t.Run("falls back to env", func(t *testing.T) {
		t.Setenv(env, "from-env")
		if got := flagOrEnv("", env); got != "from-env" {
			t.Errorf("got %q, want %q", got, "from-env")
		}
	})

	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv(env, "")
		if got := flagOrEnv("", env); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

func TestWorkspaceRefusedWithAPIToken(t *testing.T) {
	// An API token belongs to one workspace, so naming another is refused
	// before any request is made.
	t.Setenv("INVOPOP_TOKEN", "key")
	t.Setenv(workspaceEnv, "")
	root := Root()
	root.SetArgs([]string{"ping", "-w", "sandbox/shop"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot be used with an API token") {
		t.Fatalf("err = %v", err)
	}
}
