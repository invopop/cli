package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ghodss/yaml"

	"github.com/invopop/gobl/dsig"
	"github.com/invopop/gobl/org"
)

// Config represents the application configuration.
type Config struct {
	Supplier *org.Party       `json:"supplier"`
	Key      *dsig.PrivateKey `json:"key"`
	API      *API             `json:"api"`

	SeriesID string `json:"series_id"`
}

// API is some API info
type API struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

// New parses files and returns a config struct.
func New(files ...string) (*Config, error) {
	c := new(Config)

	// Determine which file exists
	var file string
	usr, _ := user.Current()
	for _, f := range files {
		if strings.HasPrefix(f, "~/") {
			f = filepath.Join(usr.HomeDir, f[2:])
		}
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			file = f
			break
		}
	}
	if file == "" {
		return nil, errors.New("no configuration file found")
	}

	if err := c.parse(file); err != nil {
		return nil, fmt.Errorf("configuration error: %w", err)
	}
	return c, nil
}

func (c *Config) parse(file string) error {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("unmarshalling yaml: %w", err)
	}

	return nil
}
