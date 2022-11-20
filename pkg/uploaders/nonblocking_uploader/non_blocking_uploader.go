package nonblocking_uploader

import (
	"context"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type NonBlockingUploader struct {
	internalDataChan   chan []byte
	WorkersReady       chan chan *domain.WorkUnit
	searchForWork      chan struct{}
	log                *zap.SugaredLogger
	dataDropper        domain.DataDropper
	filePathProvider   domain.FilePathProvider
	capacityGauge      *prometheus.GaugeVec
	workersCountGauge  *prometheus.GaugeVec
	enqueueCounter     *prometheus.CounterVec
	enqueuedItemsGauge *prometheus.GaugeVec
}

func New(
	l *zap.SugaredLogger,
	workersCount int,
	queueCapacity int,
	dataDropper domain.DataDropper,
	filePathProvider domain.FilePathProvider,
	metricRegistry *prometheus.Registry) *NonBlockingUploader {

	capacityGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "queue_capacity",
			Help:      "The total capacity of the internal queue.",
		},
		[]string{},
	)

	workersCountGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "workers_count",
			Help:      "The total number of workers, meaning how many uploads can happen in parallel.",
		},
		[]string{},
	)

	enqueueCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "enqueue_calls_total",
			Help:      "The total number of times that data was enqueued."},
		[]string{},
	)

	enqueuedItemsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "items_in_queue",
			Help:      "The count of current items in the internal queue, waiting to be uploaded.",
		},
		[]string{},
	)

	metricRegistry.MustRegister(capacityGauge, workersCountGauge, enqueueCounter, enqueuedItemsGauge)
	capacityGauge.WithLabelValues().Set(float64(queueCapacity))
	workersCountGauge.WithLabelValues().Set(float64(workersCount))

	uploader := &NonBlockingUploader{
		internalDataChan:   make(chan []byte, queueCapacity),
		WorkersReady:       make(chan chan *domain.WorkUnit, workersCount),
		searchForWork:      make(chan struct{}, 1),
		log:                l.With(logger.COMPONENT_KEY, "uploader"),
		dataDropper:        dataDropper,
		filePathProvider:   filePathProvider,
		capacityGauge:      capacityGauge,
		workersCountGauge:  workersCountGauge,
		enqueueCounter:     enqueueCounter,
		enqueuedItemsGauge: enqueuedItemsGauge,
	}

	return uploader
}

func (s *NonBlockingUploader) Enqueue(data []byte) error {
	s.enqueueCounter.WithLabelValues().Inc()

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
	s.enqueuedItemsGauge.WithLabelValues().Set(float64(itemsCount))
}
