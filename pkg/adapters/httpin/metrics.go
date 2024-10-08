package httpin

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var ensureMetricRegisteringOnce sync.Once
var sizeHist *prometheus.HistogramVec
var reqsErrorCount *prometheus.CounterVec
var decompressionLatencyHist *prometheus.HistogramVec
var decompressionCount *prometheus.CounterVec

func initializeMetrics(metricRegistry *prometheus.Registry) {

	ensureMetricRegisteringOnce.Do(func() {

		sizeHist = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "request_body_size_bytes",
				Subsystem: "http",
				Namespace: "jiboia",
				Help:      "The size in bytes of (received) request body",
				//TODO: make these buckets configurable
				Buckets: []float64{0, 1024, 524288, 1048576, 2621440, 5242880, 10485760, 52428800, 104857600},
				// 0, 1KB, 512KB, 1MB, 2.5MB, 5MB, 10MB, 50MB, 100MB
			},
			[]string{"path"},
		)

		reqsErrorCount = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "request_errors_total",
				Subsystem: "http",
				Namespace: "jiboia",
				Help:      "Information about which type of error happened on HTTP request",
			},
			[]string{"error_type", "path"},
		)

		decompressionLatencyHist = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "decompression_duration_millis",
				Subsystem: "http",
				Namespace: "jiboia",
				Help:      "The time it took to decompress the incoming payload, in milliseconds",
				Buckets:   []float64{5.0, 10.0, 25.0, 50.0, 125.0, 250.0, 500.0, 1000.0, 5000.0, 30000.0},
			},
			[]string{"path"},
		)

		decompressionCount = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "decompression_total",
				Subsystem: "http",
				Namespace: "jiboia",
				Help:      "Counter for the total decompressions performed, be it successful or not",
			},
			[]string{"type"},
		)

		metricRegistry.MustRegister(sizeHist, reqsErrorCount, decompressionLatencyHist, decompressionCount)
	})
}

func increaseErrorCount(errType string, path string) {
	reqsErrorCount.WithLabelValues(errType, path).Inc()
}

func observeSize(path string, size float64) {
	sizeHist.WithLabelValues(path).Observe(size)
}

func observeDecompressionTime(path string, elapsedTime time.Duration) {
	decompressionLatencyHist.WithLabelValues(path).Observe(float64(elapsedTime.Milliseconds()))
}

func increaseDecompressionCount(algorithm string) {
	decompressionCount.WithLabelValues(algorithm).Inc()
}
