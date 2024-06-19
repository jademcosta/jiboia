package worker

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var compressionRatioHist *prometheus.HistogramVec
var compressionLatencyHist *prometheus.HistogramVec
var ensureSingleMetricRegistration sync.Once
var workInFlightGauge *prometheus.GaugeVec

func initializeMetrics(metricRegistry *prometheus.Registry) {
	ensureSingleMetricRegistration.Do(func() {
		workInFlightGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "worker",
				Name:      "work_in_flight",
				Help:      "How many workers are performing work (vs being idle) right now.",
			},
			[]string{"flow"})

		compressionRatioHist = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "jiboia",
				Subsystem: "compression",
				Name:      "ratio",
				Help:      "the ratio of compressed size vs original size (the lower the better compression)",
				Buckets:   []float64{0.01, 0.05, 0.1, 0.15, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0, 1.1},
			}, []string{"type"})

		compressionLatencyHist = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "jiboia",
				Subsystem: "compression",
				Name:      "duration_millis",
				Help:      "The time it took to compress the payload before sending it to obj storage, in milliseconds",
				Buckets:   []float64{5.0, 10.0, 25.0, 50.0, 125.0, 250.0, 500.0, 1000.0, 5000.0, 30000.0},
			},
			[]string{"type"},
		)

		metricRegistry.MustRegister(workInFlightGauge, compressionRatioHist)
	})
}

func incWorkInFlight(flowName string) {
	workInFlightGauge.WithLabelValues(flowName).Inc()
}

func decWorkInFlight(flowName string) {
	workInFlightGauge.WithLabelValues(flowName).Dec()
}

func reportCompressionRatio(compressionType string, ratio float64) {
	compressionRatioHist.WithLabelValues(compressionType).Observe(ratio)
}

func reportCompressionDuration(compressionType string, duration time.Duration) {
	compressionLatencyHist.WithLabelValues(compressionType).Observe(float64(duration.Milliseconds()))
}
