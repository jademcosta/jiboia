package external_queue

import (
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	QUEUE_TYPE_LABEL string = "queue_type"
	NAME_LABEL       string = "name"
)

var ensureMetricRegisteringOnce sync.Once

type queueWithMetrics struct {
	wrappedQueue        uploaders.ExternalQueue
	latencyHistogram    *prometheus.HistogramVec
	enqueueCounter      *prometheus.CounterVec
	enqueueErrorCounter *prometheus.CounterVec
	wrappedType         string
	wrappedName         string
}

func NewExternalQueueWithMetrics(queue ExtQueueWithMetadata, metricRegistry *prometheus.Registry) uploaders.ExternalQueue {
	latencyHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      "put_latency_seconds",
			Subsystem: "external_queue",
			Namespace: "jiboia",
			Help:      "the time it took to finish the put action to a external queue",
			Buckets:   []float64{0.25, 0.5, 1.0, 1.5, 2.0, 5.0, 10.0, 30.0, 45.0, 60.0, 90.0, 120.0, 180.0, 240.0, 300.0, 600.0},
		},
		[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
	)

	enqueueCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "put_total",
			Namespace: "jiboia",
			Subsystem: "external_queue",
			Help:      "count of put actions to external queues that finished (successful or not)",
		},
		[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
	)

	enqueueErrorCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "put_errors_total",
			Namespace: "jiboia",
			Subsystem: "external_queue",
			Help:      "count of errors putting to external queue",
		},
		[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
	)

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(latencyHistogram, enqueueCounter, enqueueErrorCounter)
	})

	return &queueWithMetrics{
		wrappedQueue:        queue,
		latencyHistogram:    latencyHistogram,
		enqueueCounter:      enqueueCounter,
		enqueueErrorCounter: enqueueErrorCounter,
		wrappedType:         queue.Type(),
		wrappedName:         queue.Name(),
	}
}

func (w *queueWithMetrics) Enqueue(uploadResult *domain.UploadResult) error {
	startTime := time.Now()

	err := w.wrappedQueue.Enqueue(uploadResult)

	elapsepTime := time.Since(startTime).Seconds()
	w.latencyHistogram.WithLabelValues(w.wrappedType, w.wrappedName).Observe(elapsepTime)

	w.enqueueCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	if err != nil {
		w.enqueueErrorCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	}

	return err
}
