package accumulator

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const FLOW_METRIC_KEY string = "flow"

var ensureMetricRegisteringOnce sync.Once
var enqueueCounter *prometheus.CounterVec
var nextCounter *prometheus.CounterVec
var capacityGauge *prometheus.GaugeVec
var enqueuedItemsGauge *prometheus.GaugeVec

type metricCollector struct {
	flowName string
}

func NewMetricCollector(flowName string, metricRegistry *prometheus.Registry) *metricCollector {
	ensureMetricRegisteringOnce.Do(func() {
		enqueueCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: "accumulator",
				Name:      "enqueue_calls_total",
				Help:      "The total number of times that data was enqueued.",
			},
			[]string{FLOW_METRIC_KEY})

		nextCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: "accumulator",
				Name:      "next_calls_total",
				Help:      "The total number of times that data was sent to next step.",
			},
			[]string{FLOW_METRIC_KEY})

		capacityGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "accumulator",
				Name:      "queue_capacity",
				Help:      "The total capacity of the internal queue.",
			},
			[]string{FLOW_METRIC_KEY},
		)

		enqueuedItemsGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "accumulator",
				Name:      "items_in_queue",
				Help:      "The count of current items in the internal queue.",
			},
			[]string{FLOW_METRIC_KEY},
		)

		metricRegistry.MustRegister(enqueueCounter, nextCounter, capacityGauge, enqueuedItemsGauge)
	})

	return &metricCollector{
		flowName: flowName,
	}
}

func (m *metricCollector) queueCapacity(queueCapacity int) {
	capacityGauge.WithLabelValues(m.flowName).Set(float64(queueCapacity))
}

func (m *metricCollector) increaseEnqueueCounter() {
	enqueueCounter.WithLabelValues(m.flowName).Inc()
}

func (m *metricCollector) increaseNextCounter() {
	nextCounter.WithLabelValues(m.flowName).Inc()
}

func (m *metricCollector) enqueuedItems(itemsCount int) {
	enqueuedItemsGauge.WithLabelValues(m.flowName).Set(float64(itemsCount))
}
