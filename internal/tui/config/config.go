// Package config loads and persists TUI-local settings such as the saved
// API token used when the CLI hasn't supplied one.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	envToken  = "INVOPOP_TOKEN"
	envHost   = "INVOPOP_HOST"
	appDir    = "invotui"
	tokenFile = "token"
)

// Config holds the application configuration.
type Config struct {
	Token string
	Host  string
}

// Load reads the config from environment variables and config files.
func Load() *Config {
	c := &Config{
		Host: os.Getenv(envHost),
	}
	c.Token = os.Getenv(envToken)
	if c.Token == "" {
		c.Token = loadTokenFile()
	}
	return c
}

// SaveToken persists a token to the config directory.
func SaveToken(token string) error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, tokenFile), []byte(strings.TrimSpace(token)), 0600)
}

func loadTokenFile() string {
	dir, err := configDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, tokenFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func configDir() (string, error) {
	home, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, appDir), nil
}
