package config

import "time"

type CircuitBreakerConfig struct {
	TurnOn       bool  `yaml:"turn_on"`
	OpenInterval int64 `yaml:"open_interval_in_ms"`
}

func (cbConf CircuitBreakerConfig) OpenIntervalAsDuration() time.Duration {
	return time.Duration(cbConf.OpenInterval) * time.Millisecond
}
