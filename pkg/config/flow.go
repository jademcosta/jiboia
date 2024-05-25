package config

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
