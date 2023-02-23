package nonblocking_uploader

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var ensureMetricRegisteringOnce sync.Once

type metricCollector struct {
	queueCapacityGauge *prometheus.GaugeVec
	workersCountGauge  *prometheus.GaugeVec
	enqueueCounter     *prometheus.CounterVec
	enqueuedItemsGauge *prometheus.GaugeVec
}

func NewMetricCollector(metricRegistry *prometheus.Registry) *metricCollector {
	queueCapacityGauge := prometheus.NewGaugeVec(
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

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(queueCapacityGauge, workersCountGauge, enqueueCounter, enqueuedItemsGauge)
	})

	return &metricCollector{
		queueCapacityGauge: queueCapacityGauge,
		workersCountGauge:  workersCountGauge,
		enqueueCounter:     enqueueCounter,
		enqueuedItemsGauge: enqueuedItemsGauge,
	}
}

func (m *metricCollector) queueCapacity(queueCapacity int) {
	m.queueCapacityGauge.WithLabelValues().Set(float64(queueCapacity))
}

func (m *metricCollector) workersCount(workersCount int) {
	m.workersCountGauge.WithLabelValues().Set(float64(workersCount))
}

func (m *metricCollector) increaseEnqueueCounter() {
	m.enqueueCounter.WithLabelValues().Inc()
}

func (m *metricCollector) enqueuedItems(itemsCount int) {
	m.enqueuedItemsGauge.WithLabelValues().Set(float64(itemsCount))
}
