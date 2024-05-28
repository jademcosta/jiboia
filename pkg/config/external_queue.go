package config

type ExternalQueueConfig struct {
	Type   string      `yaml:"type"`
	Config interface{} `yaml:"config"`
}

func (extQConf ExternalQueueConfig) fillDefaultValues() ExternalQueueConfig {
	return extQConf
}

func (extQConf ExternalQueueConfig) validate() error {
	return nil
}
