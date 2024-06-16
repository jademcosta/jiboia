package config

import (
	"fmt"
	"slices"
)

var externalQueueConfigAllowedValues = []string{"noop", "sqs"}

type ExternalQueueConfig struct {
	Type   string      `yaml:"type"`
	Config interface{} `yaml:"config"`
}

func (extQConf ExternalQueueConfig) fillDefaultValues() ExternalQueueConfig {
	if extQConf.Type == "" {
		extQConf.Type = "noop"
	}
	return extQConf
}

func (extQConf ExternalQueueConfig) validate() error {
	if !slices.Contains(externalQueueConfigAllowedValues, extQConf.Type) {
		return fmt.Errorf("queue type must be one of %v", externalQueueConfigAllowedValues)
	}
	return nil
}
