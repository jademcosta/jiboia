package objstorage

import (
	"fmt"
	"log/slog"

	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/httpstorage"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/localstorage"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/s3"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"
)

type StorageWithMetadata interface {
	worker.ObjStorage
	Type() string
	Name() string
}

func New(l *slog.Logger, metricRegistry *prometheus.Registry, flowName string, conf *config.ObjectStorageConfig) (StorageWithMetadata, error) {

	var objStorage StorageWithMetadata
	specificConf, err := yaml.Marshal(conf.Config)
	if err != nil {
		return nil, fmt.Errorf("error parsing object storage config: %w", err)
	}

	switch conf.Type {
	case s3.TYPE:
		s3Conf, err := s3.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing s3-specific config: %w", err)
		}

		objStorage, err = s3.New(l, s3Conf)
		if err != nil {
			return nil, fmt.Errorf("error creating S3 object storage: %w", err)
		}
	case localstorage.TYPE:
		localStorageConf, err := localstorage.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing localstorage-specific config: %w", err)
		}

		objStorage, err = localstorage.New(l, localStorageConf)
		if err != nil {
			return nil, fmt.Errorf("error creating localstorage object storage: %w", err)
		}
	case httpstorage.TYPE:
		httpStorageConf, err := httpstorage.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing httpstorage-specific config: %w", err)
		}

		objStorage, err = httpstorage.NewHTTPStorage(l, httpStorageConf)
		if err != nil {
			return nil, fmt.Errorf("error creating httpstorage object storage: %w", err)
		}
	default:
		objStorage, err = nil, fmt.Errorf("invalid object storage type %s", conf.Type)
	}

	return NewStorageWithMetrics(objStorage, metricRegistry, flowName), err
}
