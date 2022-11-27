package objstorage

import (
	"fmt"

	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/localstorage"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/s3"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

const (
	s3Type           string = "s3"
	localStorageType string = "localstorage"
)

type ObjStorageWithMetadata interface {
	uploaders.ObjStorage
	Type() string
	Name() string
}

func New(l *zap.SugaredLogger, metricRegistry *prometheus.Registry, conf *config.ObjectStorage) (ObjStorageWithMetadata, error) {

	var objStorage ObjStorageWithMetadata
	specificConf, err := yaml.Marshal(conf.Config)
	if err != nil {
		return nil, fmt.Errorf("error parsing object storage config: %w", err)
	}

	switch conf.Type {
	case s3Type:
		s3Conf, err := s3.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing s3-specific config: %w", err)
		}

		objStorage, err = s3.New(l, s3Conf)
		if err != nil {
			return nil, fmt.Errorf("error creating S3 object storage: %w", err)
		}
	case localStorageType:
		localStorageConf, err := localstorage.ParseConfig(specificConf)
		if err != nil {
			return nil, fmt.Errorf("error parsing localstorage-specific config: %w", err)
		}

		objStorage, err = localstorage.New(l, localStorageConf)
		if err != nil {
			return nil, fmt.Errorf("error creating localstorage object storage: %w", err)
		}
	default:
		objStorage, err = nil, fmt.Errorf("invalid object storage type %s", conf.Type)
	}

	return NewStorageWithMetrics(objStorage, metricRegistry), err
}
