package config

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

var allowedVals map[string][]string

type Config struct {
	Log     LogConfig  `yaml:"log"`
	Version string     `yaml:"version"` //FIXME: fill the version
	Api     ApiConfig  `yaml:"api"`
	Flow    FlowConfig `yaml:"flow"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}
type ApiConfig struct {
	Port int `yaml:"port"`
}

type FlowConfig struct {
	Name                 string `yaml:"name"`
	Type                 string `yaml:"type"`
	QueueMaxSize         int    `yaml:"in_memory_queue_max_size"`
	MaxConcurrentUploads int    `yaml:"max_concurrent_uploads"`
	//TODO: Use it on workers.
	MaxRetries int `yaml:"max_retries"`
	//TODO: Use it on workers.
	Timeout       int           `yaml:"timeout"` // TODO: allow this to have a unity
	Accumulator   Accumulator   `yaml:"accumulator"`
	ExternalQueue ExternalQueue `yaml:"external_queue"`
	ObjectStorage ObjectStorage `yaml:"object_storage"`
}

type Accumulator struct {
	SizeInBytes   int    `yaml:"size_in_bytes"`
	Separator     string `yaml:"separator"`
	QueueCapacity int    `yaml:"queue_capacity"`
}

type ExternalQueue struct {
	Type   string              `yaml:"type"`
	Config ExternalQueueConfig `yaml:"config"`
}

type ExternalQueueConfig struct {
	URL       string `yaml:"url"`
	Region    string `yaml:"region"`
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
}

type ObjectStorage struct {
	Type   string              `yaml:"type"`
	Config ObjectStorageConfig `yaml:"config"`
}

type ObjectStorageConfig struct {
	Bucket         string `yaml:"bucket"`
	Region         string `yaml:"region"`
	Endpoint       string `yaml:"endpoint"`
	AccessKey      string `yaml:"access_key"`
	SecretKey      string `yaml:"secret_key"`
	Prefix         string `yaml:"prefix"`
	ForcePathStyle bool   `yaml:"force_path_style"`
}

func New(confData []byte) (*Config, error) {
	c := &Config{
		Log: LogConfig{
			Level:  "info",
			Format: "json",
		},

		Api: ApiConfig{
			Port: 9010,
		},

		Flow: FlowConfig{
			Timeout:              30,
			MaxConcurrentUploads: 500,
		},
	}

	err := yaml.Unmarshal(confData, &c)
	if err != nil {
		return nil, err
	}

	err = validateConfig(c)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func validateConfig(c *Config) error {

	if !allowed(allowedValues("log.level"), c.Log.Level) {
		panic(fmt.Sprintf("log level should be one of %v", allowedValues("log.level")))
	}
	return nil
}

func allowed(group []string, elem string) bool {
	for _, a := range group {
		if a == elem {
			return true
		}
	}
	return false
}

func allowedValues(key string) []string {
	if len(allowedVals) == 0 {
		allowedVals = map[string][]string{
			"log.level": {"debug", "info", "warn", "error"},
		}
	}
	return allowedVals[key]
}
