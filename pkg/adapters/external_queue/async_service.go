package external_queue

import (
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"go.uber.org/zap"
)

// const (
// 	sqsType     string = "sqs"
// 	kinesisType string = "kinesis"
// 	noopType    string = "noop"
// )

type ExtQueueWithMetadata interface {
	uploaders.ExternalQueue
	Type() string
	Name() string
}

func New(l *zap.SugaredLogger, conf *config.Config) (uploaders.ExternalQueue, error) {

	// switch asyncServiceType {
	// case sqsType:
	// 	c := &sqs.Config{}
	// 	v.Unmarshal(c)
	// 	externalAsyncService, err = sqs.New(l, c)
	// case kinesisType:
	// 	c := &kinesis.Config{}
	// 	v.Unmarshal(c)
	// 	externalAsyncService, err = kinesis.New(l, c)
	// case noopType:
	// 	externalAsyncService = noop_async_service.New(l)
	// 	err = nil
	// default:
	// 	externalAsyncService, err = nil, fmt.Errorf("invalid service type %s", asyncServiceType)
	// }

	// if err != nil {
	// 	return nil, fmt.Errorf("error creating external async service: %w", err)
	// }
	return nil, nil
}
