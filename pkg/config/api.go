package config

import (
	"errors"
	"fmt"
)

const DefaultPort = 9199

type ApiConfig struct {
	Port             int    `yaml:"port"`
	PayloadSizeLimit string `yaml:"payload_size_limit"`
}

func (apiConf ApiConfig) fillDefaults() ApiConfig {
	if apiConf.Port == 0 {
		apiConf.Port = DefaultPort
	}

	return apiConf
}

func (apiConf ApiConfig) validate() error {

	if apiConf.PayloadSizeLimit != "" {
		_, err := ToBytes(apiConf.PayloadSizeLimit)
		if err != nil {
			return fmt.Errorf("invalid payload size limite: %w", err)
		}
	}

	if apiConf.Port == 0 {
		return errors.New("api.port cannot be 0")
	}
	return nil
}

func (apiconf ApiConfig) PayloadSizeLimitInBytes() (int64, error) {
	return ToBytes(apiconf.PayloadSizeLimit)
}
