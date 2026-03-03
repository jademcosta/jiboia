package config

type ObjectStorageConfig struct {
	Type   string      `json:"type"   yaml:"type"`
	Config interface{} `json:"config" yaml:"config"`
}

func (objStgConf ObjectStorageConfig) fillDefaultValues() ObjectStorageConfig {
	return objStgConf
}

func (objStgConf ObjectStorageConfig) validate() error { //Validated further in the factory
	//TODO: validate empty type
	return nil
}
