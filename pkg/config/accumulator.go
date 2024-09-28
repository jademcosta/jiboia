package config

import "fmt"

type AccumulatorConfig struct {
	Size           string               `yaml:"size"`
	Separator      string               `yaml:"separator"`
	QueueCapacity  int                  `yaml:"queue_capacity"` //TODO: validate and test
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

func (accConf AccumulatorConfig) SizeAsBytes() (int64, error) {
	return ToBytes(accConf.Size)
}

func (accConf AccumulatorConfig) fillDefaultValues() AccumulatorConfig {
	accConf.CircuitBreaker = accConf.CircuitBreaker.fillDefaultValues()
	return accConf
}

func (accConf AccumulatorConfig) validate() error {
	err := accConf.CircuitBreaker.validate()
	if err != nil {
		return fmt.Errorf("on acumulator: %w", err)
	}
	return nil
}
