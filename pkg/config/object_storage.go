package config

type ObjectStorageConfig struct {
	Type   string      `yaml:"type"`
	Config interface{} `yaml:"config"`
}
