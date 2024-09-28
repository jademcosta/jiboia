package config

import (
	"errors"
	"fmt"
	"strings"
)

type FlowConfig struct {
	Name                 string              `yaml:"name"`
	QueueMaxSize         int                 `yaml:"in_memory_queue_max_size"`
	MaxConcurrentUploads int                 `yaml:"max_concurrent_uploads"`
	PathPrefixCount      int                 `yaml:"path_prefix_count"`
	Ingestion            IngestionConfig     `yaml:"ingestion"`
	Accumulator          AccumulatorConfig   `yaml:"accumulator"`
	ExternalQueue        ExternalQueueConfig `yaml:"external_queue"`
	ObjectStorage        ObjectStorageConfig `yaml:"object_storage"`
	Compression          CompressionConfig   `yaml:"compression"`
}

func (flwConf FlowConfig) fillDefaultValues() FlowConfig {
	if flwConf.MaxConcurrentUploads <= 0 {
		flwConf.MaxConcurrentUploads = 50
	}

	if flwConf.PathPrefixCount <= 0 {
		flwConf.PathPrefixCount = 1
	}

	flwConf.Ingestion = flwConf.Ingestion.fillDefaultValues()
	flwConf.Accumulator = flwConf.Accumulator.fillDefaultValues()
	flwConf.ExternalQueue = flwConf.ExternalQueue.fillDefaultValues()
	flwConf.ObjectStorage = flwConf.ObjectStorage.fillDefaultValues()
	flwConf.Compression = flwConf.Compression.fillDefaultValues()
	return flwConf
}

func (flwConf FlowConfig) validate() error {
	if flwConf.Name == "" {
		return fmt.Errorf("all flows must have a name")
	}

	flowNameContainsSpace := strings.Contains(flwConf.Name, " ")
	if flowNameContainsSpace {
		return fmt.Errorf("flow name must not have spaces")
	}

	if flwConf.PathPrefixCount == 0 {
		return errors.New("path prefix count cannot be zero")
	}

	err := flwConf.Ingestion.validate()
	if err != nil {
		return err
	}
	err = flwConf.Accumulator.validate()
	if err != nil {
		return err
	}
	err = flwConf.ExternalQueue.validate()
	if err != nil {
		return err
	}
	err = flwConf.ObjectStorage.validate()
	if err != nil {
		return err
	}
	err = flwConf.Compression.validate()
	if err != nil {
		return err
	}
	return nil
}
