package httpmiddleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type metricsMiddleware struct {
	reqsCount   *prometheus.CounterVec
	latencyHist *prometheus.HistogramVec
	next        http.Handler
}

func NewMetricsMiddleware(appName string, metricRegistry *prometheus.Registry) func(next http.Handler) http.Handler {
	midd := &metricsMiddleware{}

	midd.reqsCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "requests_total",
			Subsystem:   "http",
			Namespace:   "jiboia",
			Help:        "How many HTTP requests processed, partitioned by status code, method and HTTP path.",
			ConstLabels: prometheus.Labels{"service": appName},
		},
		[]string{"code", "method", "path"},
	)

	//TODO: once we have the sync route method, add 180.0, 240.0, 300.0 and 600.0 buckets on histogram
	midd.latencyHist = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "request_duration_seconds",
			Subsystem:   "http",
			Namespace:   "jiboia",
			Help:        "Latency of HTTP requests, in seconds.",
			ConstLabels: prometheus.Labels{"service": appName},
			Buckets:     []float64{0.1, 0.2, 0.5, 1.0, 2.5, 5.0, 10.0, 15.0, 30.0, 60.0, 120.0},
		},
		[]string{"path"}, //TODO: should I add code? Might turn this into too many metrics...
	)

	metricRegistry.MustRegister(midd.reqsCount, midd.latencyHist)

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
	midd.latencyHist.WithLabelValues(r.URL.Path).Observe(latency)
	midd.reqsCount.WithLabelValues(strconv.Itoa(wrapper.statusCode), r.Method, r.URL.Path).Inc()
}
