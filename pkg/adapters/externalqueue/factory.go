package externalqueue

import (
	"fmt"
	"log/slog"

	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue/noopqueue"
	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue/sqs"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

type ExtQueueWithMetadata interface {
	worker.ExternalQueue
	Type() string
	Name() string
}

func New(
	l *slog.Logger, metricRegistry *prometheus.Registry, flowName string, conf *config.ExternalQueueConfig,
) (ExtQueueWithMetadata, error) {

	var externalQueue ExtQueueWithMetadata
	specificConf, err := yaml.Marshal(conf.Config)
	if err != nil {
		return nil, fmt.Errorf("error parsing external queue config: %w", err)
	}

	switch conf.Type {
	case noopqueue.TYPE:
		externalQueue = noopqueue.New(l)
	case sqs.TYPE:
		c, err := sqs.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing SQS-specific config: %w", err)
		}

		externalQueue, err = sqs.New(l, c, flowName)
		if err != nil {
			return nil, fmt.Errorf("error creating SQS: %w", err)
		}
	default:
		externalQueue, err = nil, fmt.Errorf("invalid external queue type %s", conf.Type)
	}

	return NewExternalQueueWithMetrics(externalQueue, metricRegistry, flowName), err
}
