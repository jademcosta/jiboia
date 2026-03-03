package config

import "fmt"

type IngestionConfig struct {
	Token          string               `json:"token"           yaml:"token"`
	Decompression  DescompressionConfig `json:"decompress"      yaml:"decompress"`
	CircuitBreaker CircuitBreakerConfig `json:"circuit_breaker" yaml:"circuit_breaker"`
}

func (ingConf IngestionConfig) redacted() IngestionConfig {
	redactedIngestionConf := ingConf
	if redactedIngestionConf.Token != "" {
		redactedIngestionConf.Token = RedactedValue
	}
	return redactedIngestionConf
}

func (ingConf IngestionConfig) fillDefaultValues() IngestionConfig {
	ingConf.CircuitBreaker = ingConf.CircuitBreaker.fillDefaultValues()
	ingConf.Decompression = ingConf.Decompression.fillDefaultValues()
	return ingConf
}

func (ingConf IngestionConfig) validate() error {
	err := ingConf.CircuitBreaker.validate()
	if err != nil {
		return fmt.Errorf("on ingestion: %w", err)
	}

	err = ingConf.Decompression.validate()
	if err != nil {
		return err
	}

	return nil
}
