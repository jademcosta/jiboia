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
}

func NewMetricCollector(flowName string, metricRegistry *prometheus.Registry) *metricCollector {
	enqueueCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "jiboia",
			Subsystem:   "accumulator",
			Name:        "enqueue_calls_total",
			Help:        "The total number of times that data was enqueued.",
			ConstLabels: prometheus.Labels{FLOW_METRIC_KEY: flowName},
		},
		[]string{})

	nextCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace:   "jiboia",
			Subsystem:   "accumulator",
			Name:        "next_calls_total",
			Help:        "The total number of times that data was sent to next step.",
			ConstLabels: prometheus.Labels{FLOW_METRIC_KEY: flowName},
		},
		[]string{})

	capacityGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "jiboia",
			Subsystem:   "accumulator",
			Name:        "queue_capacity",
			Help:        "The total capacity of the internal queue.",
			ConstLabels: prometheus.Labels{FLOW_METRIC_KEY: flowName},
		},
		[]string{},
	)

	enqueuedItemsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace:   "jiboia",
			Subsystem:   "accumulator",
			Name:        "items_in_queue",
			Help:        "The count of current items in the internal queue.",
			ConstLabels: prometheus.Labels{FLOW_METRIC_KEY: flowName},
		},
		[]string{},
	)

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(enqueueCounter, nextCounter, capacityGauge, enqueuedItemsGauge)
	})

	return &metricCollector{
		enqueueCounter:     enqueueCounter,
		nextCounter:        nextCounter,
		capacityGauge:      capacityGauge,
		enqueuedItemsGauge: enqueuedItemsGauge,
	}
}

func (m *metricCollector) queueCapacity(queueCapacity int) {
	m.capacityGauge.WithLabelValues().Set(float64(queueCapacity))
}

func (m *metricCollector) increaseEnqueueCounter() {
	m.enqueueCounter.WithLabelValues().Inc()
}

func (m *metricCollector) increaseNextCounter() {
	m.nextCounter.WithLabelValues().Inc()
}

func (m *metricCollector) enqueuedItems(itemsCount int) {
	m.enqueuedItemsGauge.WithLabelValues().Set(float64(itemsCount))
}
