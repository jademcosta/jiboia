package config

type ExternalQueueConfig struct {
	Type   string      `yaml:"type"`
	Config interface{} `yaml:"config"`
}
