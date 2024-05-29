package config

import (
	"fmt"
)

type DescompressionConfig struct {
	ActiveDecompressions []string `yaml:"active"`
	MaxConcurrency       int      `yaml:"max_concurrency"`
}

func (decompConf DescompressionConfig) fillDefaultValues() DescompressionConfig {
	return decompConf
}

func (decompConf DescompressionConfig) validate() error {

	if len(decompConf.ActiveDecompressions) == 0 {
		return nil
	}

	for _, decompressionType := range decompConf.ActiveDecompressions {
		if !allowed(allowedValues("compression"), decompressionType) {
			return fmt.Errorf("ingestion.decompress.active option must be one of %v",
				allowedValues("compression"))
		}
	}

	return nil
}
