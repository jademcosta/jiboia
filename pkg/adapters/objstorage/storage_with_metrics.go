package objstorage

import (
	"sync"
	"time"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	STORAGE_TYPE_LABEL string = "storage_type"
	NAME_LABEL         string = "name"
)

var ensureMetricRegisteringOnce sync.Once

type storageWithMetrics struct {
	storage              worker.ObjStorage
	latencyHistogram     *prometheus.HistogramVec
	uploadCounter        *prometheus.CounterVec
	uploadSuccessCounter *prometheus.CounterVec
	uploadErrorCounter   *prometheus.CounterVec
	wrappedType          string
	wrappedName          string
}

func NewStorageWithMetrics(storage ObjStorageWithMetadata, metricRegistry *prometheus.Registry) ObjStorageWithMetadata {
	latencyHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      "upload_latency_seconds",
			Subsystem: "object_storage",
			Namespace: "jiboia",
			Help:      "the time it took to finish the upload of data to object storage",
			Buckets:   []float64{0.25, 0.5, 1.0, 1.5, 2.0, 5.0, 10.0, 30.0, 45.0, 60.0, 90.0, 120.0, 180.0, 240.0, 300.0, 600.0},
		},
		[]string{STORAGE_TYPE_LABEL, NAME_LABEL},
	)

	uploadCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "upload_total",
			Namespace: "jiboia",
			Subsystem: "object_storage",
			Help:      "count of uploads to object storage that finished",
		},
		[]string{STORAGE_TYPE_LABEL, NAME_LABEL},
	)

	uploadSuccessCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "upload_success_total",
			Namespace: "jiboia",
			Subsystem: "object_storage",
			Help:      "count of successes uploading to object storage",
		},
		[]string{STORAGE_TYPE_LABEL, NAME_LABEL},
	)

	uploadErrorCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:      "upload_errors_total",
			Namespace: "jiboia",
			Subsystem: "object_storage",
			Help:      "count of errors uploading to object storage",
		},
		[]string{STORAGE_TYPE_LABEL, NAME_LABEL},
	)

	ensureMetricRegisteringOnce.Do(func() {
		metricRegistry.MustRegister(
			latencyHistogram,
			uploadCounter,
			uploadSuccessCounter,
			uploadErrorCounter,
		)
	})

	return &storageWithMetrics{
		storage:              storage,
		latencyHistogram:     latencyHistogram,
		uploadCounter:        uploadCounter,
		uploadSuccessCounter: uploadSuccessCounter,
		uploadErrorCounter:   uploadErrorCounter,
		wrappedType:          storage.Type(),
		wrappedName:          storage.Type(),
	}
}

func (w *storageWithMetrics) Upload(workU *domain.WorkUnit) (*domain.UploadResult, error) {
	startTime := time.Now()

	uploadResult, err := w.storage.Upload(workU)
	elapsedTime := time.Since(startTime).Seconds()

	w.latencyHistogram.
		WithLabelValues(w.wrappedType, w.wrappedName).
		Observe(elapsedTime)

	w.uploadCounter.
		WithLabelValues(w.wrappedType, w.wrappedName).
		Inc()

	if err != nil {
		w.uploadErrorCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	} else {
		//TODO: do we need a label with error type?
		w.uploadSuccessCounter.WithLabelValues(w.wrappedType, w.wrappedName).Inc()
	}
	return uploadResult, err
}

func (w *storageWithMetrics) Type() string {
	return w.wrappedType
}

func (w *storageWithMetrics) Name() string {
	return w.wrappedName
}
