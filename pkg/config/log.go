package config

import (
	"fmt"
	"slices"
)

var allowedLogConfigLevels = []string{"debug", "info", "warn", "error"}

type LogConfig struct {
	Level  string `json:"level"  yaml:"level"`
	Format string `json:"format" yaml:"format"`
}

func (logConf LogConfig) fillDefaults() LogConfig {
	if logConf.Level == "" {
		logConf.Level = "info"
	}

	if logConf.Format == "" {
		logConf.Format = "json"
	}

	return logConf
}

func (logConf LogConfig) validate() error {
	if !slices.Contains(allowedLogConfigLevels, logConf.Level) {
		return fmt.Errorf("log level should be one of %v", allowedLogConfigLevels)
	}

	return nil
}
