package config

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

type Config struct {
	O11y            O11yConfig   `yaml:"o11y"`
	Version         string       `yaml:"version"` //FIXME: fill the version
	Api             ApiConfig    `yaml:"api"`
	Flows           []FlowConfig `yaml:"flows"`
	DisableMaxProcs bool         `yaml:"disable_max_procs"`
}

func New(confData []byte) (*Config, error) {
	c := &Config{
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

	err = c.O11y.validate()
	if err != nil {
		return err
	}

	return nil
}

func (c *Config) fillDefaultValues() {

	c.O11y = c.O11y.fillDefaults()
	c.Api = c.Api.fillDefaults()

	for idx, flow := range c.Flows {
		c.Flows[idx] = flow.fillDefaultValues()
	}
}
