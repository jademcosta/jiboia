package nonblocking_uploader

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const FLOW_METRIC_KEY string = "flow"

var ensureMetricRegisteringOnce sync.Once

type metricCollector struct {
	queueCapacityGauge *prometheus.GaugeVec
	workersCountGauge  *prometheus.GaugeVec
	enqueueCounter     *prometheus.CounterVec
	enqueuedItemsGauge *prometheus.GaugeVec
	flowName           string
}

func NewMetricCollector(flowName string, metricRegistry *prometheus.Registry) *metricCollector {
	queueCapacityGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "queue_capacity",
			Help:      "The total capacity of the internal queue.",
		},
		[]string{FLOW_METRIC_KEY},
	)

	workersCountGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "workers_count",
			Help:      "The total number of workers, meaning how many uploads can happen in parallel.",
		},
		[]string{FLOW_METRIC_KEY},
	)

	enqueueCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "enqueue_calls_total",
			Help:      "The total number of times that data was enqueued.",
		},
		[]string{FLOW_METRIC_KEY},
	)

	enqueuedItemsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "uploader",
			Name:      "items_in_queue",
			Help:      "The count of current items in the internal queue, waiting to be uploaded.",
		},
		[]string{FLOW_METRIC_KEY},
	)

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(queueCapacityGauge, workersCountGauge, enqueueCounter, enqueuedItemsGauge)
	})

	return &metricCollector{
		queueCapacityGauge: queueCapacityGauge,
		workersCountGauge:  workersCountGauge,
		enqueueCounter:     enqueueCounter,
		enqueuedItemsGauge: enqueuedItemsGauge,
		flowName:           flowName,
	}
}

func (m *metricCollector) queueCapacity(queueCapacity int) {
	m.queueCapacityGauge.WithLabelValues(m.flowName).Set(float64(queueCapacity))
}

func (m *metricCollector) workersCount(workersCount int) {
	m.workersCountGauge.WithLabelValues(m.flowName).Set(float64(workersCount))
}

func (m *metricCollector) increaseEnqueueCounter() {
	m.enqueueCounter.WithLabelValues(m.flowName).Inc()
}

func (m *metricCollector) enqueuedItems(itemsCount int) {
	m.enqueuedItemsGauge.WithLabelValues(m.flowName).Set(float64(itemsCount))
}
