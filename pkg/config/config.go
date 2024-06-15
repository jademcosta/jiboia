package config

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

var allowedVals map[string][]string

func init() {
	allowedVals = map[string][]string{
		"log.level": {"debug", "info", "warn", "error"},
		//TODO: delete these
		"compression":       {"gzip", "zlib", "deflate", "snappy", "zstd"},
		"compression.level": {"1", "2", "3", "4", "5", "6", "7", "8", "9"},
	}
}

type Config struct {
	Log             LogConfig    `yaml:"log"`
	Version         string       `yaml:"version"` //FIXME: fill the version
	Api             ApiConfig    `yaml:"api"`
	Flows           []FlowConfig `yaml:"flows"`
	DisableMaxProcs bool         `yaml:"disable_max_procs"`
}

func New(confData []byte) (*Config, error) {
	c := &Config{
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},

		Api: ApiConfig{
			Port: 9010,
		},
	}

	err := yaml.Unmarshal(confData, &c)
	if err != nil {
		return nil, err
	}

	c.fillDefaultValues()

	err = c.validate()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) validate() error {

	if len(c.Flows) <= 0 {
		return fmt.Errorf("at least one flow should be declared")
	}

	flowNamesSet := make(map[string]struct{})
	for _, flow := range c.Flows {

		if _, exists := flowNamesSet[flow.Name]; exists {
			return fmt.Errorf("flow names must be unique")
		}
		flowNamesSet[flow.Name] = struct{}{}

		err := flow.validate()
		if err != nil {
			return err
		}
	}

	err := c.Api.validate()
	if err != nil {
		return err
	}

	err = c.Log.validate()
	if err != nil {
		return err
	}

	return nil
}

func allowed(group []string, elem string) bool {
	for _, a := range group {
		if a == elem {
			return true
		}
	}
	return false
}

func allowedValues(key string) []string {
	return allowedVals[key]
}

func (c *Config) fillDefaultValues() {

	c.Log = c.Log.fillDefaults()
	c.Api = c.Api.fillDefaults()

	for idx, flow := range c.Flows {
		c.Flows[idx] = flow.fillDefaultValues()
	}
}
