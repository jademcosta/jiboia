package config

type AccumulatorConfig struct {
	SizeInBytes    int                  `yaml:"size_in_bytes"`
	Separator      string               `yaml:"separator"`
	QueueCapacity  int                  `yaml:"queue_capacity"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

func (accConf AccumulatorConfig) fillDefaultValues() AccumulatorConfig {
	accConf.CircuitBreaker = accConf.CircuitBreaker.fillDefaultValues()
	return accConf
}

func (accConf AccumulatorConfig) validate() error {
	err := accConf.CircuitBreaker.validate()
	if err != nil {
		return err
	}
	return nil
}
