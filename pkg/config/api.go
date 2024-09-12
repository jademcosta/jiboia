package config

import (
	"errors"
	"fmt"
)

const DefaultPort = 9199

type APIConfig struct {
	Port             int    `yaml:"port"`
	PayloadSizeLimit string `yaml:"payload_size_limit"`
}

func (apiConf APIConfig) fillDefaults() APIConfig {
	if apiConf.Port == 0 {
		apiConf.Port = DefaultPort
	}

	return apiConf
}

func (apiConf APIConfig) validate() error {

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

func (apiConf APIConfig) PayloadSizeLimitInBytes() (int64, error) {
	return ToBytes(apiConf.PayloadSizeLimit)
}
