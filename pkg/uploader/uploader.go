package uploader

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

type NonBlockingUploader struct {
	internalDataChan chan []byte
	next             chan *domain.WorkUnit
	searchForWork    chan struct{}
	log              *slog.Logger
	filePathProvider domain.FilePathProvider
	metrics          *metricCollector
	shutdownMutex    sync.RWMutex
	shuttingDown     bool
	ctx              context.Context
	enqueueHappening atomic.Int64
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
		internalDataChan: make(chan []byte, queueCapacity),
		next:             next,
		searchForWork:    make(chan struct{}, 1),
		log:              l.With(logger.ComponentKey, "uploader"),
		filePathProvider: filePathProvider,
		metrics:          metrics,
	}

	return uploader
}

func (s *NonBlockingUploader) Enqueue(data []byte) error {
	s.shutdownMutex.RLock()
	defer s.shutdownMutex.RUnlock()
	if s.shuttingDown {
		return errors.New("uploader shutting down")
	}

	s.enqueueHappening.Add(1)
	defer s.enqueueHappening.Add(-1)
	s.metrics.increaseEnqueueCounter()

	select {
	case s.internalDataChan <- data:
		s.updateEnqueuedItemsMetric()
	default:
		s.metrics.incEnqueueFailed()
		return errors.New("enqueueing data to accumulate on uploader failed, queue is full")
	}

	return nil
}

// Run should be called in a new goroutine
func (s *NonBlockingUploader) Run(ctx context.Context) {
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

func (s *NonBlockingUploader) sendWorkToNext(data []byte) {
	workU := &domain.WorkUnit{
		Filename: *s.filePathProvider.Filename(),
		Prefix:   *s.filePathProvider.Prefix(),
		Data:     data,
	}

	s.next <- workU
	s.updateEnqueuedItemsMetric()
}

func (s *NonBlockingUploader) updateEnqueuedItemsMetric() {
	itemsCount := len(s.internalDataChan)
	s.metrics.enqueuedItems(itemsCount)
}

func (s *NonBlockingUploader) shutdown() {
	s.setShutdown()
	s.waitAllEnqueuesToFinish()
	close(s.internalDataChan)

	for data := range s.internalDataChan {
		s.sendWorkToNext(data)
	}
	close(s.next)
}

func (s *NonBlockingUploader) setShutdown() {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()
	s.shuttingDown = true
}

func (s *NonBlockingUploader) waitAllEnqueuesToFinish() {
	pendingCounter := s.enqueueHappening.Load()
	for pendingCounter != 0 {
		time.Sleep(10 * time.Millisecond)
	}
}
