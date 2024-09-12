package objstorage

import (
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	StorageTypeLabel string = "storage_type"
	NameLabel        string = "name"
	FlowLabel        string = "flow"
)

var (
	ensureMetricRegisteringOnce sync.Once
	latencyHistogram            *prometheus.HistogramVec
	uploadCounter               *prometheus.CounterVec
	uploadSuccessCounter        *prometheus.CounterVec
	uploadErrorCounter          *prometheus.CounterVec
)

type storageWithMetrics struct {
	storage     worker.ObjStorage
	wrappedType string
	wrappedName string
	name        string
}

func NewStorageWithMetrics(storage StorageWithMetadata, metricRegistry *prometheus.Registry, name string) StorageWithMetadata {
	ensureMetricRegisteringOnce.Do(func() {
		latencyHistogram = prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:      "upload_latency_seconds",
				Subsystem: "object_storage",
				Namespace: "jiboia",
				Help:      "the time it took to finish the upload of data to object storage",
				Buckets:   []float64{0.25, 0.5, 1.0, 1.5, 2.0, 5.0, 10.0, 30.0, 45.0, 60.0, 90.0, 120.0, 180.0, 240.0, 300.0, 600.0},
			},
			[]string{StorageTypeLabel, NameLabel, FlowLabel},
		)

		uploadCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "upload_total",
				Namespace: "jiboia",
				Subsystem: "object_storage",
				Help:      "count of uploads to object storage that finished",
			},
			[]string{StorageTypeLabel, NameLabel, FlowLabel},
		)

		uploadSuccessCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "upload_success_total",
				Namespace: "jiboia",
				Subsystem: "object_storage",
				Help:      "count of successes uploading to object storage",
			},
			[]string{StorageTypeLabel, NameLabel, FlowLabel},
		)

		uploadErrorCounter = prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name:      "upload_errors_total",
				Namespace: "jiboia",
				Subsystem: "object_storage",
				Help:      "count of errors uploading to object storage",
			},
			[]string{StorageTypeLabel, NameLabel, FlowLabel},
		)

		metricRegistry.MustRegister(
			latencyHistogram,
			uploadCounter,
			uploadSuccessCounter,
			uploadErrorCounter,
		)
	})

	return &storageWithMetrics{
		storage:     storage,
		name:        name,
		wrappedType: storage.Type(),
		wrappedName: storage.Name(),
	}
}

func (w *storageWithMetrics) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	startTime := time.Now()

	uploadResult, err := w.storage.Upload(workU)
	elapsedTime := time.Since(startTime).Seconds()

	latencyHistogram.
		WithLabelValues(w.wrappedType, w.wrappedName, w.name).
		Observe(elapsedTime)

	uploadCounter.
		WithLabelValues(w.wrappedType, w.wrappedName, w.name).
		Inc()

	if err != nil {
		uploadErrorCounter.WithLabelValues(w.wrappedType, w.wrappedName, w.name).Inc()
	} else {
		//TODO: do we need a label with error type?
		uploadSuccessCounter.WithLabelValues(w.wrappedType, w.wrappedName, w.name).Inc()
	}
	return uploadResult, err
}

func (w *storageWithMetrics) Type() string {
	return w.wrappedType
}

func (w *storageWithMetrics) Name() string {
	return w.wrappedName
}
