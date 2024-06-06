package config

import (
	"errors"
	"fmt"
)

const defaultInitialBufferSize = "512"

type DescompressionConfig struct {
	ActiveDecompressions []string `yaml:"active"`
	MaxConcurrency       int      `yaml:"max_concurrency"`
	InitialBufferSize    string   `yaml:"initial_buffer_size"`
}

func (decompConf DescompressionConfig) InitialBufferSizeAsBytes() (int64, error) {
	return ToBytes(decompConf.InitialBufferSize)
}

func (decompConf DescompressionConfig) fillDefaultValues() DescompressionConfig {
	if decompConf.InitialBufferSize == "" {
		decompConf.InitialBufferSize = defaultInitialBufferSize
	}
	return decompConf
}

func (decompConf DescompressionConfig) validate() error {

	if len(decompConf.ActiveDecompressions) == 0 {
		return nil
	}

	if decompConf.InitialBufferSize == "" {
		return errors.New("initial_buffer_size cannot be zero")
	}

	for _, decompressionType := range decompConf.ActiveDecompressions {
		if !allowed(allowedValues("compression"), decompressionType) {
			return fmt.Errorf("ingestion.decompress.active option must be one of %v",
				allowedValues("compression"))
		}
	}

	return nil
}
