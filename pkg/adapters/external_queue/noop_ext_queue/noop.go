package noop_ext_queue

import (
	"github.com/jademcosta/jiboia/pkg/domain"
	"go.uber.org/zap"
)

const TYPE = "noop"
const NAME = "noop"

type NoopExternalQueue struct {
	log *zap.SugaredLogger
}

func New(l *zap.SugaredLogger) *NoopExternalQueue {
	return &NoopExternalQueue{
		log: l,
	}
}

func (noop *NoopExternalQueue) Enqueue(uploadResult *domain.MessageContext) error {
	noop.log.Debugw("enqueue called on No-op ext queue", "url", uploadResult.URL)
	return nil
}

func (noop *NoopExternalQueue) Type() string {
	return TYPE
}

func (noop *NoopExternalQueue) Name() string {
	return NAME
}
