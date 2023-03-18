package non_blocking_bucket

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const FLOW_METRIC_KEY string = "flow"

var ensureMetricRegisteringOnce sync.Once

type metricCollector struct {
	enqueueCounter     *prometheus.CounterVec
	nextCounter        *prometheus.CounterVec
	capacityGauge      *prometheus.GaugeVec
	enqueuedItemsGauge *prometheus.GaugeVec
	flowName           string
}

func NewMetricCollector(flowName string, metricRegistry *prometheus.Registry) *metricCollector {
	enqueueCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "enqueue_calls_total",
			Help:      "The total number of times that data was enqueued.",
		},
		[]string{FLOW_METRIC_KEY})

	nextCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "next_calls_total",
			Help:      "The total number of times that data was sent to next step.",
		},
		[]string{FLOW_METRIC_KEY})

	capacityGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "queue_capacity",
			Help:      "The total capacity of the internal queue.",
		},
		[]string{FLOW_METRIC_KEY},
	)

	enqueuedItemsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "accumulator",
			Name:      "items_in_queue",
			Help:      "The count of current items in the internal queue.",
		},
		[]string{FLOW_METRIC_KEY},
	)

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(enqueueCounter, nextCounter, capacityGauge, enqueuedItemsGauge)
	})

	return &metricCollector{
		enqueueCounter:     enqueueCounter,
		nextCounter:        nextCounter,
		capacityGauge:      capacityGauge,
		enqueuedItemsGauge: enqueuedItemsGauge,
		flowName:           flowName,
	}
}

func (m *metricCollector) queueCapacity(queueCapacity int) {
	m.capacityGauge.WithLabelValues(m.flowName).Set(float64(queueCapacity))
}

func (m *metricCollector) increaseEnqueueCounter() {
	m.enqueueCounter.WithLabelValues(m.flowName).Inc()
}

func (m *metricCollector) increaseNextCounter() {
	m.nextCounter.WithLabelValues(m.flowName).Inc()
}

func (m *metricCollector) enqueuedItems(itemsCount int) {
	m.enqueuedItemsGauge.WithLabelValues(m.flowName).Set(float64(itemsCount))
}
