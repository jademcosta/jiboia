package http_in

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var ensureMetricRegisteringOnce sync.Once
var sizeHist *prometheus.HistogramVec
var reqsErrorCount *prometheus.CounterVec

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

		metricRegistry.MustRegister(sizeHist, reqsErrorCount)
	})
}

func increaseErrorCount(errType string, path string) {
	reqsErrorCount.WithLabelValues(errType, path)
}

func observeSize(path string, size float64) {
	sizeHist.WithLabelValues(path).Observe(size)
}
