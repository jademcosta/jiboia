package config

import (
	"errors"
	"fmt"
)

const DefaultPreallocSlicePercentage = 2

type CompressionConfig struct {
	Level                   string `yaml:"level"`
	Type                    string `yaml:"type"`
	PreallocSlicePercentage int    `yaml:"prealloc_slice_percentage"`
}

func (compConf CompressionConfig) fillDefaultValues() CompressionConfig {
	if compConf.PreallocSlicePercentage == 0 {
		compConf.PreallocSlicePercentage = DefaultPreallocSlicePercentage
	}
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

	if compConf.PreallocSlicePercentage <= 0 || compConf.PreallocSlicePercentage > 100 {
		return errors.New("prealloc_slice_percentage should be in the interval [0,100)")
	}

	return nil
}
