package external_queue

import (
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	QUEUE_TYPE_LABEL string = "queue_type"
	NAME_LABEL       string = "name"
)

var (
	ensureMetricRegisteringOnce sync.Once
	latencyHistogram            *prometheus.HistogramVec
	enqueueCounter              *prometheus.CounterVec
	enqueueErrorCounter         *prometheus.CounterVec
	enqueueSuccessCounter       *prometheus.CounterVec
)

type queueWithMetrics struct {
	wrappedQueue worker.ExternalQueue
	wrappedType  string
	wrappedName  string
}

func NewExternalQueueWithMetrics(queue ExtQueueWithMetadata, metricRegistry *prometheus.Registry) ExtQueueWithMetadata {
	ensureMetricRegisteringOnce.Do(func() {
		latencyHistogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "put_latency_seconds",
				Subsystem: "external_queue",
				Namespace: "jiboia",
				Help:      "the time it took to finish the put action to a external queue (only successful cases)",
				Buckets:   []float64{0.25, 0.5, 1.0, 1.5, 2.0, 5.0, 10.0, 30.0, 45.0, 60.0},
			},
			[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
		)

		enqueueCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "put_total",
				Namespace: "jiboia",
				Subsystem: "external_queue",
				Help:      "count of put actions to external queues that finished (successful or not)",
			},
			[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
		)

		enqueueErrorCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "put_errors_total",
				Namespace: "jiboia",
				Subsystem: "external_queue",
				Help:      "count of errors putting to external queue",
			},
			[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
		)

		enqueueSuccessCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "put_success_total",
				Namespace: "jiboia",
				Subsystem: "external_queue",
				Help:      "count of successes putting to external queue",
			},
			[]string{QUEUE_TYPE_LABEL, NAME_LABEL},
		)

		metricRegistry.MustRegister(latencyHistogram, enqueueCounter, enqueueErrorCounter, enqueueSuccessCounter)
	})

	return &queueWithMetrics{
		wrappedQueue: queue,
		wrappedType:  queue.Type(),
		wrappedName:  queue.Name(),
	}
}

func (w *queueWithMetrics) Enqueue(uploadResult *domain.UploadResult) error {
	enqueueCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	startTime := time.Now()

	err := w.wrappedQueue.Enqueue(uploadResult)
	elapsepTime := time.Since(startTime).Seconds()

	if err != nil {
		enqueueErrorCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	} else {
		latencyHistogram.WithLabelValues(w.wrappedType, w.wrappedName).Observe(elapsepTime)
		enqueueSuccessCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	}

	return err
}

func (w *queueWithMetrics) Type() string {
	return w.wrappedType
}

func (w *queueWithMetrics) Name() string {
	return w.wrappedName
}
