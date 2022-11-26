package external_queue

import (
	"fmt"

	"github.com/jademcosta/jiboia/pkg/adapters/external_queue/sqs"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const (
	sqsType string = "sqs"
	// kinesisType string = "kinesis"
	// noopType    string = "noop"
)

type ExtQueueWithMetadata interface {
	uploaders.ExternalQueue
	Type() string
	Name() string
}

func New(l *zap.SugaredLogger, conf *config.ExternalQueue) (ExtQueueWithMetadata, error) {

	var externalQueue ExtQueueWithMetadata
	specificConf, err := yaml.Marshal(conf.Config)
	if err != nil {
		return nil, fmt.Errorf("error parsing external queue config: %w", err)
	}

	switch conf.Type {
	case sqsType:
		c, err := sqs.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing SQS-specific config: %w", err)
		}

		externalQueue, err = sqs.New(l, c)
		if err != nil {
			return nil, fmt.Errorf("error creating SQS: %w", err)
		}
	default:
		externalQueue, err = nil, fmt.Errorf("invalid external queue type %s", conf.Type)
	}

	// BUG: missing the metrics wrapper here

	return externalQueue, err
}
