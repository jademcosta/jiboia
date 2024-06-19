package config

import (
	"fmt"
	"slices"
)

var allowedLogConfigLevels = []string{"debug", "info", "warn", "error"}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func (logConf LogConfig) fillDefaults() LogConfig {
	if logConf.Level == "" {
		logConf.Level = "info"
	}

	return logConf
}

func (logConf LogConfig) validate() error {
	if !slices.Contains(allowedLogConfigLevels, logConf.Level) {
		return fmt.Errorf("log level should be one of %v", allowedLogConfigLevels)
	}

	return nil
}
