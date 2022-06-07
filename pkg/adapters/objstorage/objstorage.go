package objstorage

import (
	"fmt"

	"github.com/jademcosta/jiboia/pkg/adapters/objstorage/s3"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"go.uber.org/zap"
)

const (
	s3Type string = "s3"
)

type ObjStorageWithMetadata interface {
	uploaders.ObjStorage
	Type() string
	Name() string
}

func New(l *zap.SugaredLogger, conf *config.Config) (ObjStorageWithMetadata, error) {

	var objStorage ObjStorageWithMetadata
	var err error

	switch conf.Flow.ObjectStorage.Type {
	case s3Type:
		objStorage, err = s3.New(l, &conf.Flow.ObjectStorage.Config)
	default:
		objStorage, err = nil, fmt.Errorf("invalid object storage type %s", conf.Flow.ObjectStorage.Type)
	}

	return objStorage, err
}
