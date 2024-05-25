package config

type DescompressionConfig struct {
	ActiveDecompressions []string `yaml:"active"`
	MaxConcurrency       int      `yaml:"max_concurrency"`
}

type IngestionConfig struct {
	Token         string               `yaml:"token"`
	Decompression DescompressionConfig `yaml:"decompress"`
}
