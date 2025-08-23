package uploader

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

type NonBlockingUploader struct {
	internalDataChan chan *domain.WorkUnit
	next             chan *domain.WorkUnit
	searchForWork    chan struct{}
	log              *slog.Logger
	filePathProvider domain.FilePathProvider
	metrics          *metricCollector
	shutdownMutex    sync.RWMutex
	shuttingDown     bool
	ctx              context.Context
	doneChan         chan struct{}
	doneChanMu       sync.Mutex
}

func New(
	flowName string,
	l *slog.Logger,
	workersCount int,
	queueCapacity int,
	filePathProvider domain.FilePathProvider,
	metricRegistry *prometheus.Registry,
	next chan *domain.WorkUnit,

) *NonBlockingUploader {

	metrics := newMetricCollector(flowName, metricRegistry)

	metrics.queueCapacity(queueCapacity)
	metrics.workersCount(workersCount)

	uploader := &NonBlockingUploader{
		internalDataChan: make(chan *domain.WorkUnit, queueCapacity),
		next:             next,
		searchForWork:    make(chan struct{}, 1),
		log:              l.With(logger.ComponentKey, "uploader"),
		filePathProvider: filePathProvider,
		metrics:          metrics,
	}

	return uploader
}

func (s *NonBlockingUploader) Enqueue(payload *domain.WorkUnit) error {
	s.shutdownMutex.RLock()
	defer s.shutdownMutex.RUnlock()
	if s.shuttingDown {
		return errors.New("uploader shutting down")
	}

	s.metrics.increaseEnqueueCounter()

	select {
	case s.internalDataChan <- payload:
		s.updateEnqueuedItemsMetric()
	default:
		s.metrics.incEnqueueFailed()
		return errors.New("enqueueing data to accumulate on uploader failed, queue is full")
	}

	return nil
}

// Run should be called in a new goroutine
func (s *NonBlockingUploader) Run(ctx context.Context) {
	s.doneChanMu.Lock()
	s.doneChan = make(chan struct{})
	defer close(s.doneChan)
	s.doneChanMu.Unlock()

	s.log.Info("starting non-blocking uploader loop")
	s.ctx = ctx
	for {
		select {
		case <-ctx.Done():
			s.log.Debug("uploader starting shutdown")
			s.shutdown()
			s.log.Info("uploader shutdown finished")
			return
		case payload := <-s.internalDataChan:
			s.sendWorkToNext(payload)
		}
	}
}

func (s *NonBlockingUploader) Done() <-chan struct{} {
	s.doneChanMu.Lock()
	defer s.doneChanMu.Unlock()
	return s.doneChan
}

func (s *NonBlockingUploader) sendWorkToNext(payload *domain.WorkUnit) {
	payload.Filename = *s.filePathProvider.Filename()
	payload.Prefix = *s.filePathProvider.Prefix()

	s.next <- payload
	s.updateEnqueuedItemsMetric()
}

func (s *NonBlockingUploader) updateEnqueuedItemsMetric() {
	itemsCount := len(s.internalDataChan)
	s.metrics.enqueuedItems(itemsCount)
}

func (s *NonBlockingUploader) shutdown() {
	s.setShutdown()
	close(s.internalDataChan)

	for {
		data, moreWork := <-s.internalDataChan
		if !moreWork {
			break
		}
		s.sendWorkToNext(data)
	}
	close(s.next)
}

func (s *NonBlockingUploader) setShutdown() {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()
	s.shuttingDown = true
}
