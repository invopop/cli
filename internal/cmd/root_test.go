package cmd

import "testing"

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
