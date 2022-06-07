package noop_external_queue

import "go.uber.org/zap"

type NoopAsyncService struct {
	log *zap.SugaredLogger
}

func New(l *zap.SugaredLogger) *NoopAsyncService {
	return &NoopAsyncService{
		log: l,
	}
}

func (noop *NoopAsyncService) Enqueue(dataPath *string) error {
	noop.log.Debug("Enqueue called on No-op async service queue", "data_path", dataPath)
	return nil
}
