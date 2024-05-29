package noop_ext_queue

import (
	"log/slog"

	"github.com/jademcosta/jiboia/pkg/domain"
)

const TYPE = "noop"
const NAME = "noop"

type NoopExternalQueue struct {
	log *slog.Logger
}

func New(l *slog.Logger) *NoopExternalQueue {
	return &NoopExternalQueue{
		log: l,
	}
}

func (noop *NoopExternalQueue) Enqueue(uploadResult *domain.MessageContext) error {
	noop.log.Debug("enqueue called on No-op ext queue", "url", uploadResult.URL)
	return nil
}

func (noop *NoopExternalQueue) Type() string {
	return TYPE
}

func (noop *NoopExternalQueue) Name() string {
	return NAME
}
