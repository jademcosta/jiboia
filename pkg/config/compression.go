package config

import "fmt"

type CompressionConfig struct {
	Level string `yaml:"level"`
	Type  string `yaml:"type"`
}

func (compConf CompressionConfig) fillDefaultValues() CompressionConfig {
	return compConf
}

func (compConf CompressionConfig) validate() error {
	if compConf.Type == "" {
		return nil
	}

	if !allowed(allowedValues("compression"), compConf.Type) {
		return fmt.Errorf("compression.type option must be one of %v",
			allowedValues("compression"))
	}

	return nil
}
