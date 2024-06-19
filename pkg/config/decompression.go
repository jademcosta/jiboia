package config

import (
	"errors"
	"fmt"
	"slices"
)

const defaultInitialBufferSize = "512"

var allowedDecompressions = []string{"gzip", "zlib", "deflate", "zstd", "snappy"}

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

	if decompConf.MaxConcurrency < 0 {
		return errors.New("max concurrency cannot be < 0")
	}

	for _, decompressionType := range decompConf.ActiveDecompressions {
		if !slices.Contains(allowedDecompressions, decompressionType) {
			return fmt.Errorf("decompression type option must be one of %v", allowedDecompressions)
		}
	}
	return nil
}
