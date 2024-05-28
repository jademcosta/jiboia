package config

type CompressionConfig struct {
	Level string `yaml:"level"`
	Type  string `yaml:"type"`
}

func (compssConf CompressionConfig) fillDefaultValues() CompressionConfig {
	return compssConf
}

func (compssConf CompressionConfig) validate() error {
	return nil
}
