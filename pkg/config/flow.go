package config

import (
	"errors"
	"fmt"
	"strings"
)

type FlowConfig struct {
	Name                 string                `json:"name"                     yaml:"name"`
	QueueMaxSize         int                   `json:"in_memory_queue_max_size" yaml:"in_memory_queue_max_size"`
	MaxConcurrentUploads int                   `json:"max_concurrent_uploads"   yaml:"max_concurrent_uploads"`
	PathPrefixCount      int                   `json:"path_prefix_count"        yaml:"path_prefix_count"`
	Ingestion            IngestionConfig       `json:"ingestion"                yaml:"ingestion"`
	Accumulator          AccumulatorConfig     `json:"accumulator"              yaml:"accumulator"`
	ExternalQueues       []ExternalQueueConfig `json:"external_queues"          yaml:"external_queues"`
	ObjectStorages       []ObjectStorageConfig `json:"object_storages"          yaml:"object_storages"`
	Compression          CompressionConfig     `json:"compression"              yaml:"compression"`
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
	flwConf.Compression = flwConf.Compression.fillDefaultValues()

	for idx, objStorage := range flwConf.ObjectStorages {
		flwConf.ObjectStorages[idx] = objStorage.fillDefaultValues()
	}

	for idx, extQueue := range flwConf.ExternalQueues {
		flwConf.ExternalQueues[idx] = extQueue.fillDefaultValues()
	}

	return flwConf
}

func (flwConf FlowConfig) redacted() FlowConfig {
	redacted := flwConf
	redacted.Ingestion = flwConf.Ingestion.redacted()
	return redacted
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
	err = flwConf.Compression.validate()
	if err != nil {
		return err
	}

	for _, objStorage := range flwConf.ObjectStorages {
		err = objStorage.validate()
		if err != nil {
			return err
		}
	}

	for _, extQueue := range flwConf.ExternalQueues {
		err = extQueue.validate()
		if err != nil {
			return err
		}
	}

	return nil
}
