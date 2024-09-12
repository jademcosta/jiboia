package uploader

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const FLOW_METRIC_KEY string = "flow"

var ensureMetricRegisteringOnce sync.Once

var queueCapacityGauge *prometheus.GaugeVec
var workersCountGauge *prometheus.GaugeVec
var enqueueCounter *prometheus.CounterVec
var enqueuedItemsGauge *prometheus.GaugeVec
var enqueueFailed *prometheus.CounterVec

type metricCollector struct {
	flowName string
}

func NewMetricCollector(flowName string, metricRegistry *prometheus.Registry) *metricCollector {
	ensureMetricRegisteringOnce.Do(func() {
		queueCapacityGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "uploader",
				Name:      "queue_capacity",
				Help:      "The total capacity of the internal queue.",
			},
			[]string{FLOW_METRIC_KEY},
		)

		workersCountGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "uploader",
				Name:      "workers_online",
				Help:      "The total number of workers, meaning how many uploads can happen in parallel.",
			},
			[]string{FLOW_METRIC_KEY},
		)

		enqueueCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: "uploader",
				Name:      "enqueue_calls_total",
				Help:      "The total number of times that data was enqueued.",
			},
			[]string{FLOW_METRIC_KEY},
		)

		enqueuedItemsGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "uploader",
				Name:      "items_in_queue",
				Help:      "The count of current items in the internal queue, waiting to be uploaded.",
			},
			[]string{FLOW_METRIC_KEY},
		)

		enqueueFailed = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: "uploader",
				Name:      "enqueue_failed_total",
				Help:      "Counter for failures when trying to enqueue data on it",
			},
			[]string{FLOW_METRIC_KEY})

		metricRegistry.MustRegister(
			queueCapacityGauge, workersCountGauge, enqueueCounter, enqueuedItemsGauge,
			enqueueFailed,
		)
	})

	return &metricCollector{
		flowName: flowName,
	}
}

func (m *metricCollector) queueCapacity(queueCapacity int) {
	queueCapacityGauge.WithLabelValues(m.flowName).Set(float64(queueCapacity))
}

func (m *metricCollector) workersCount(workersCount int) {
	workersCountGauge.WithLabelValues(m.flowName).Set(float64(workersCount))
}

func (m *metricCollector) increaseEnqueueCounter() {
	enqueueCounter.WithLabelValues(m.flowName).Inc()
}

func (m *metricCollector) enqueuedItems(itemsCount int) {
	enqueuedItemsGauge.WithLabelValues(m.flowName).Set(float64(itemsCount))
}

func (m *metricCollector) incEnqueueFailed() {
	enqueueFailed.WithLabelValues(m.flowName).Inc()
}
