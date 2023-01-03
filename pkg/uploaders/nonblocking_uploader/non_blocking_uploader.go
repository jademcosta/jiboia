package nonblocking_uploader

import (
	"context"

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
}

func New(
	l *zap.SugaredLogger,
	workersCount int,
	queueCapacity int,
	dataDropper domain.DataDropper,
	filePathProvider domain.FilePathProvider,
	metricRegistry *prometheus.Registry) *NonBlockingUploader {

	m := NewMetricCollector(metricRegistry)

	m.queueCapacity(queueCapacity)
	m.workersCount(workersCount)

	uploader := &NonBlockingUploader{
		internalDataChan: make(chan []byte, queueCapacity),
		WorkersReady:     make(chan chan *domain.WorkUnit, workersCount),
		searchForWork:    make(chan struct{}, 1),
		log:              l.With(logger.COMPONENT_KEY, "uploader"),
		dataDropper:      dataDropper,
		filePathProvider: filePathProvider,
		metrics:          m,
	}

	return uploader
}

func (s *NonBlockingUploader) Enqueue(data []byte) error {
	s.metrics.increaseEnqueueCounter()

	select {
	case s.internalDataChan <- data:
		s.updateEnqueuedItemsMetric()
	default:
		s.dataDropped(data)
	}

	return nil
}

//Run should be called in a new goroutine
func (s *NonBlockingUploader) Run(ctx context.Context) {
	s.log.Info("Starting non-blocking uploader loop")
	for {
		select {
		case worker := <-s.WorkersReady:
			s.sendWork(worker)
			s.updateEnqueuedItemsMetric()
		case <-ctx.Done():
			//TODO: implenment graceful shutdown
			return
		}
	}
}

func (s *NonBlockingUploader) sendWork(worker chan *domain.WorkUnit) {
	data := <-s.internalDataChan
	workU := &domain.WorkUnit{
		Filename: *s.filePathProvider.Filename(),
		Prefix:   *s.filePathProvider.Prefix(),
		Data:     data,
	}

	worker <- workU
}

func (s *NonBlockingUploader) dataDropped(data []byte) {
	s.dataDropper.Drop(data)
}

func (s *NonBlockingUploader) updateEnqueuedItemsMetric() {
	itemsCount := len(s.internalDataChan)
	s.metrics.enqueuedItems(itemsCount)
}
