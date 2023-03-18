package nonblocking_uploader

import (
	"context"
	"errors"
	"sync"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type NonBlockingUploader struct {
	internalDataChan chan []byte
	WorkersReady     chan chan *domain.WorkUnit
	searchForWork    chan struct{}
	log              *zap.SugaredLogger
	dataDropper      domain.DataDropper
	filePathProvider domain.FilePathProvider
	metrics          *metricCollector
	shutdownMutex    sync.RWMutex
	shuttingDown     bool
	workersCount     int
	ctx              context.Context
}

func New(
	flowName string,
	l *zap.SugaredLogger,
	workersCount int,
	queueCapacity int,
	dataDropper domain.DataDropper,
	filePathProvider domain.FilePathProvider,
	metricRegistry *prometheus.Registry) *NonBlockingUploader {

	metrics := NewMetricCollector(flowName, metricRegistry)

	metrics.queueCapacity(queueCapacity)
	metrics.workersCount(workersCount)

	uploader := &NonBlockingUploader{
		internalDataChan: make(chan []byte, queueCapacity),
		WorkersReady:     make(chan chan *domain.WorkUnit, workersCount),
		searchForWork:    make(chan struct{}, 1),
		log:              l.With(logger.COMPONENT_KEY, "uploader"),
		dataDropper:      dataDropper,
		filePathProvider: filePathProvider,
		metrics:          metrics,
		workersCount:     workersCount,
	}

	return uploader
}

func (s *NonBlockingUploader) Enqueue(data []byte) error {
	s.shutdownMutex.RLock()
	defer s.shutdownMutex.RUnlock()
	if s.shuttingDown {
		return errors.New("uploader shutting down")
	}

	s.metrics.increaseEnqueueCounter()

	select {
	case s.internalDataChan <- data:
		s.updateEnqueuedItemsMetric()
	default:
		s.dataDropped(data)
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
		case worker := <-s.WorkersReady:
			s.sendWork(worker)
		case <-ctx.Done():
			s.log.Debug("uploader starting shutdown")
			s.shutdown()
			s.log.Info("uploader shutdown finished")
			return
		}
	}
}

func (s *NonBlockingUploader) sendWork(worker chan *domain.WorkUnit) {
	select {
	case <-s.ctx.Done(): //TODO: this is ugly. We shopuldn't need to hear for done on 2 places
		s.WorkersReady <- worker
		return
	case data := <-s.internalDataChan:
		workU := &domain.WorkUnit{
			Filename: *s.filePathProvider.Filename(),
			Prefix:   *s.filePathProvider.Prefix(),
			Data:     data,
		}

		worker <- workU
		s.updateEnqueuedItemsMetric()
	}
}

func (s *NonBlockingUploader) dataDropped(data []byte) {
	s.dataDropper.Drop(data)
}

func (s *NonBlockingUploader) updateEnqueuedItemsMetric() {
	itemsCount := len(s.internalDataChan)
	s.metrics.enqueuedItems(itemsCount)
}

func (s *NonBlockingUploader) shutdown() {
	s.setShutdown()

	workerShutdownCounter := 0

	for {
		worker := <-s.WorkersReady
		dataPendingExists := len(s.internalDataChan) > 0

		if dataPendingExists {
			s.sendWork(worker)
		} else {
			workerShutdownCounter++
			allWorkersShutdown := workerShutdownCounter == s.workersCount

			if allWorkersShutdown {
				// close(s.WorkersReady)
				// close(s.internalDataChan)
				//TODO (jademcosta): this throws a race error. The problem is that close() needs
				// synchronization structures. On the other hand, I don't wanna make workers use a Mutex
				// right now, so I'm leaving the channels without being closed.
				return
			}
		}
	}
}

func (s *NonBlockingUploader) setShutdown() {
	s.shutdownMutex.Lock()
	defer s.shutdownMutex.Unlock()
	s.shuttingDown = true
}
