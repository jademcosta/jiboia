package config

type AccumulatorConfig struct {
	SizeInBytes    int                  `yaml:"size_in_bytes"`
	Separator      string               `yaml:"separator"`
	QueueCapacity  int                  `yaml:"queue_capacity"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}
