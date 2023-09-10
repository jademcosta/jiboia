package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v2"
)

var allowedVals map[string][]string

func init() {
	allowedVals = map[string][]string{
		"log.level": {"debug", "info", "warn", "error"},
		//TODO: merge strings duplicated here and on compressor package
		"compression":       {"gzip", "zlib", "deflate", "snappy", "zstd"},
		"compression.level": {"1", "2", "3", "4", "5", "6", "7", "8", "9"},
	}
}

type Config struct {
	Log     LogConfig    `yaml:"log"`
	Version string       `yaml:"version"` //FIXME: fill the version
	Api     ApiConfig    `yaml:"api"`
	Flows   []FlowConfig `yaml:"flows"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type FlowConfig struct {
	Name                 string          `yaml:"name"`
	QueueMaxSize         int             `yaml:"in_memory_queue_max_size"`
	MaxConcurrentUploads int             `yaml:"max_concurrent_uploads"`
	PathPrefixCount      int             `yaml:"path_prefix_count"`
	Ingestion            IngestionConfig `yaml:"ingestion"`
	Accumulator          Accumulator     `yaml:"accumulator"`
	ExternalQueue        ExternalQueue   `yaml:"external_queue"`
	ObjectStorage        ObjectStorage   `yaml:"object_storage"`
	Compression          Compression     `yaml:"compression"`
}

type Compression struct {
	Level string `yaml:"level"`
	Type  string `yaml:"type"`
}

type Accumulator struct {
	SizeInBytes    int               `yaml:"size_in_bytes"`
	Separator      string            `yaml:"separator"`
	QueueCapacity  int               `yaml:"queue_capacity"`
	CircuitBreaker map[string]string `yaml:"circuit_breaker"`
}

type ExternalQueue struct {
	Type   string      `yaml:"type"`
	Config interface{} `yaml:"config"`
}

type ObjectStorage struct {
	Type   string      `yaml:"type"`
	Config interface{} `yaml:"config"`
}

type IngestionConfig struct {
	Token           string   `yaml:"token"`
	DecompressTypes []string `yaml:"decompress_ingested_data"`
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
	}

	err := yaml.Unmarshal(confData, &c)
	if err != nil {
		return nil, err
	}

	err = validateConfig(c)
	if err != nil {
		return nil, err
	}

	fillFlowsDefaultValues(c)
	return c, nil
}

func validateConfig(c *Config) error {

	if !allowed(allowedValues("log.level"), c.Log.Level) {
		return fmt.Errorf("log level should be one of %v", allowedValues("log.level"))
	}

	if len(c.Flows) <= 0 {
		return fmt.Errorf("at least one flow should be declared")
	}

	flowNamesSet := make(map[string]struct{})
	for _, flow := range c.Flows {
		if flow.Name == "" {
			return fmt.Errorf("all flows must have a name")
		}

		flowNameContainsSpace := strings.Contains(flow.Name, " ")
		if flowNameContainsSpace {
			return fmt.Errorf("flow name must not have spaces")
		}

		if _, exists := flowNamesSet[flow.Name]; exists {
			return fmt.Errorf("flow names must be unique")
		}

		flowNamesSet[flow.Name] = struct{}{}

		if flow.Compression.Type != "" {
			if !allowed(allowedValues("compression"), flow.Compression.Type) {
				return fmt.Errorf("compression type should be one of %v", allowedValues("compression"))
			}

			if flow.Compression.Level != "" {
				if !allowed(allowedValues("compression.level"), flow.Compression.Level) {
					return fmt.Errorf("compression level should be one of %v", allowedValues("compression.level"))
				}
			}
		}

		if len(flow.Ingestion.DecompressTypes) > 0 {
			for _, decompressionType := range flow.Ingestion.DecompressTypes {
				if !allowed(allowedValues("compression"), decompressionType) {
					return fmt.Errorf("ingestion.decompress_ingested_data option should be one of %v",
						allowedValues("compression"))
				}
			}
		}
	}

	err := c.Api.validateSizeLimit()
	if err != nil {
		return err
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
	return allowedVals[key]
}

func fillFlowsDefaultValues(c *Config) {

	for idx, flow := range c.Flows {
		if flow.MaxConcurrentUploads <= 0 {
			c.Flows[idx].MaxConcurrentUploads = 500
		}

		if flow.PathPrefixCount <= 0 {
			c.Flows[idx].PathPrefixCount = 1
		}
	}
}
