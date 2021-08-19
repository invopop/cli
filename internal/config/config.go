package config

import (
	"fmt"
	"io/ioutil"
	"path"

	"github.com/ghodss/yaml"
	"github.com/invopop/gobl/dsig"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/region"
)

type Config struct {
	Region   region.Code      `json:"region"`
	Supplier *org.Party       `json:"supplier"`
	Key      *dsig.PrivateKey `json:"key"`
	API      *API             `json:"api"`

	SeriesID string `json:"series_id"`
}

type API struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

func New(file string) *Config {
	c := new(Config)
	if err := c.parse(file); err != nil {
		panic("configuration error: " + err.Error())
	}
	return c
}

func (c *Config) parse(file string) error {
	f := path.Join(".", file)

	data, err := ioutil.ReadFile(f)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("unmarshalling yaml: %w", err)
	}

	return nil
}
