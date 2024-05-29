package config

import (
	"errors"
	"time"
)

const DefaultOpenInterval = 100

type CircuitBreakerConfig struct {
	Disable      bool  `yaml:"disable"`
	OpenInterval int64 `yaml:"open_interval_in_ms"`
}

func (cbConf CircuitBreakerConfig) OpenIntervalAsDuration() time.Duration {
	return time.Duration(cbConf.OpenInterval) * time.Millisecond
}

func (cbConf CircuitBreakerConfig) fillDefaultValues() CircuitBreakerConfig {
	if cbConf.OpenInterval == 0 {
		cbConf.OpenInterval = DefaultOpenInterval
	}
	return cbConf
}

func (cbConf CircuitBreakerConfig) validate() error {
	if cbConf.OpenInterval <= 0 {
		return errors.New("open_interval cannot be <= 0")
	}
	return nil
}
