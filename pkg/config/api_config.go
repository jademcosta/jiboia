package config

import (
	"errors"
	"fmt"
)

type ApiConfig struct {
	Port             int    `yaml:"port"`
	PayloadSizeLimit string `yaml:"payload_size_limit"`
}

func (apiConf ApiConfig) fillDefaults() ApiConfig {
	return apiConf
}

func (apiConf ApiConfig) validate() error {

	err := apiConf.validateSizeLimit()
	if err != nil {
		return err
	}

	if apiConf.Port == 0 {
		return errors.New("api.port cannot be 0")
	}
	return nil
}

func (apiconf ApiConfig) PayloadSizeLimitInBytes() (int64, error) {
	return ToBytes(apiconf.PayloadSizeLimit)
}

func (apiconf ApiConfig) validateSizeLimit() error {
	_, err := ToBytes(apiconf.PayloadSizeLimit)
	if err != nil {
		return fmt.Errorf("invalid payload size limite: %w", err)
	}

	return nil
}
