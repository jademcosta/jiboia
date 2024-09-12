package accumulator

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const FlowMetricKey string = "flow"

var ensureMetricRegisteringOnce sync.Once
var enqueueCounter *prometheus.CounterVec
var nextCounter *prometheus.CounterVec
var capacityGauge *prometheus.GaugeVec
var enqueuedItemsGauge *prometheus.GaugeVec
var dataSizeInBytesCounter *prometheus.CounterVec
var dataSizeOutBytesCounter *prometheus.CounterVec
var dataSizeInKBsCounter *prometheus.CounterVec
var dataSizeOutKBsCounter *prometheus.CounterVec
var enqueueFailed *prometheus.CounterVec

type metricCollector struct {
	flowName string
}

func newMetricCollector(flowName string, metricRegistry *prometheus.Registry) *metricCollector {
	ensureMetricRegisteringOnce.Do(func() {
		enqueueCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "enqueue_calls_total",
				Help:      "The total number of times that data was enqueued.",
			},
			[]string{FlowMetricKey})

		nextCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "next_calls_total",
				Help:      "The total number of times that data was sent to next step.",
			},
			[]string{FlowMetricKey})

		capacityGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "queue_capacity",
				Help:      "The total capacity of the internal queue.",
			},
			[]string{FlowMetricKey},
		)

		enqueuedItemsGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "items_in_queue",
				Help:      "The count of current items in the internal queue.",
			},
			[]string{FlowMetricKey},
		)

		dataSizeInBytesCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "data_in_bytes",
				Help:      "The amount of data that has been worked by accumulator component, in bytes.",
			},
			[]string{FlowMetricKey})

		dataSizeOutBytesCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "data_out_bytes",
				Help:      "The amount of data that has been sent forward (to the next compoenent) by accumulator component, in bytes.",
			},
			[]string{FlowMetricKey})

		dataSizeInKBsCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "data_in_kbs",
				Help:      "The amount of data that has been worked by accumulator component, in KBs.",
			},
			[]string{FlowMetricKey})

		dataSizeOutKBsCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "data_out_kbs",
				Help:      "The amount of data that has been sent forward (to the next compoenent) by accumulator component, in KBs.",
			},
			[]string{FlowMetricKey})

		enqueueFailed = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "jiboia",
				Subsystem: ComponentName,
				Name:      "enqueue_failed_total",
				Help:      "Counter for failures when trying to enqueue data on it",
			},
			[]string{FlowMetricKey})

		metricRegistry.MustRegister(
			enqueueCounter,
			nextCounter,
			capacityGauge,
			enqueuedItemsGauge,
			dataSizeInBytesCounter,
			dataSizeOutBytesCounter,
			dataSizeInKBsCounter,
			dataSizeOutKBsCounter,
			enqueueFailed)
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

func (m *metricCollector) incDataInBytesBy(size int) {
	if size > 0 {
		dataSizeInBytesCounter.WithLabelValues(m.flowName).Add(float64(size))
		dataSizeInKBsCounter.WithLabelValues(m.flowName).Add(float64(size) / 1024)
	}
}

func (m *metricCollector) incDataOutBytesBy(size int) {
	if size > 0 {
		dataSizeOutBytesCounter.WithLabelValues(m.flowName).Add(float64(size))
		dataSizeOutKBsCounter.WithLabelValues(m.flowName).Add(float64(size) / 1024)
	}
}

func (m *metricCollector) incEnqueueFailed() {
	enqueueFailed.WithLabelValues(m.flowName).Inc()
}
