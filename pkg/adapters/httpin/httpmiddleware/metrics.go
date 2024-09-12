package httpmiddleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var ensureMetricRegisteringOnce sync.Once
var reqsCount *prometheus.CounterVec
var latencyHist *prometheus.HistogramVec

type metricsMiddleware struct {
	next http.Handler
}

func NewMetricsMiddleware(metricRegistry *prometheus.Registry) func(next http.Handler) http.Handler {
	midd := &metricsMiddleware{}

	ensureMetricRegisteringOnce.Do(func() {
		reqsCount = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "requests_total",
				Subsystem: "http",
				Namespace: "jiboia",
				Help:      "How many HTTP requests processed.",
			},
			[]string{"code", "method", "path"},
		)

		//TODO: once we have the sync route method, add 180.0, 240.0, 300.0 and 600.0 buckets on histogram
		latencyHist = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "request_duration_seconds",
				Subsystem: "http",
				Namespace: "jiboia",
				Help:      "Latency of HTTP requests, in seconds.",
				Buckets:   []float64{0.1, 0.2, 0.5, 1.0, 2.5, 5.0, 10.0, 15.0, 30.0, 60.0, 120.0},
			},
			[]string{"path"},
		)

		metricRegistry.MustRegister(reqsCount, latencyHist)
	})

	return func(next http.Handler) http.Handler {
		midd.next = next
		return midd
	}
}

func (midd *metricsMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	timeStart := time.Now()
	wrapper := &responseWriterWrapper{wrapped: w}

	midd.next.ServeHTTP(wrapper, r)

	latency := time.Since(timeStart).Seconds()
	latencyHist.WithLabelValues(r.URL.Path).Observe(latency)
	reqsCount.WithLabelValues(strconv.Itoa(wrapper.statusCode), r.Method, r.URL.Path).Inc()
}
