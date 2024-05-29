package config

import "fmt"

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

func (logConf LogConfig) fillDefaults() LogConfig {
	return logConf
}

func (logConf LogConfig) validate() error {
	if !allowed(allowedValues("log.level"), logConf.Level) {
		return fmt.Errorf("log level should be one of %v", allowedValues("log.level"))
	}

	return nil
}
