package config

type IngestionConfig struct {
	Token          string               `yaml:"token"`
	Decompression  DescompressionConfig `yaml:"decompress"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

func (ingConf IngestionConfig) fillDefaultValues() IngestionConfig {
	ingConf.CircuitBreaker = ingConf.CircuitBreaker.fillDefaultValues()
	ingConf.Decompression = ingConf.Decompression.fillDefaultValues()
	return ingConf
}

func (ingConf IngestionConfig) validate() error {
	err := ingConf.CircuitBreaker.validate()
	if err != nil {
		return err
	}

	err = ingConf.Decompression.validate()
	if err != nil {
		return err
	}

	return nil
}
