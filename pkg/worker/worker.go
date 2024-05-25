package worker

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/jademcosta/jiboia/pkg/compressor"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var ensureSingleMetricRegistration sync.Once
var workInFlightGauge *prometheus.GaugeVec
var compressionRatioHist *prometheus.HistogramVec

type ObjStorage interface {
	Upload(workU *domain.WorkUnit) (*domain.UploadResult, error)
}

type ExternalQueue interface {
	Enqueue(*domain.MessageContext) error
}

type Worker struct {
	l                    *zap.SugaredLogger
	workChan             chan *domain.WorkUnit
	storage              ObjStorage
	queue                ExternalQueue
	workVolunteeringChan chan chan *domain.WorkUnit
	flowName             string
	compressionConf      config.CompressionConfig
}

func NewWorker(
	flowName string,
	l *zap.SugaredLogger,
	storage ObjStorage,
	extQueue ExternalQueue,
	workVolunteeringChan chan chan *domain.WorkUnit,
	metricRegistry *prometheus.Registry,
	compressionConf config.CompressionConfig) *Worker {

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

		metricRegistry.MustRegister(workInFlightGauge, compressionRatioHist)
	})

	workChan := make(chan *domain.WorkUnit, 1)
	return &Worker{
		l:                    l.With(logger.COMPONENT_KEY, "worker"),
		storage:              storage,
		queue:                extQueue,
		workVolunteeringChan: workVolunteeringChan,
		workChan:             workChan,
		flowName:             flowName,
		compressionConf:      compressionConf,
	}
}

// Run should be called on a goroutine
func (w *Worker) Run(ctx context.Context) {
	for {
		w.workVolunteeringChan <- w.workChan
		select {
		case workU := <-w.workChan:
			w.work(workU)
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) work(workU *domain.WorkUnit) {
	workInFlightGauge.WithLabelValues(w.flowName).Inc()
	defer workInFlightGauge.WithLabelValues(w.flowName).Dec()

	compressedData, err := compress(w.compressionConf, workU.Data)
	if err != nil {
		w.l.Errorw("error compressing data", "prefix", workU.Prefix, "filename", workU.Filename, "error", err)
		return
	}
	compressionRatioHist.WithLabelValues(w.compressionConf.Type).Observe(
		float64(len(compressedData)) / float64(len(workU.Data)))

	workU.Data = compressedData
	uploadResult, err := w.storage.Upload(workU)

	if err != nil {
		w.l.Errorw("failed to upload object", "prefix", workU.Prefix, "filename", workU.Filename, "error", err)
		return
	} else {
		w.l.Debugw("finished uploading object", "prefix", workU.Prefix, "filename", workU.Filename)
	}

	msgContext := &domain.MessageContext{
		Bucket:          uploadResult.Bucket,
		Region:          uploadResult.Region,
		Path:            uploadResult.Path,
		URL:             uploadResult.URL,
		SizeInBytes:     uploadResult.SizeInBytes,
		CompressionType: w.compressionConf.Type,
	}
	err = w.queue.Enqueue(msgContext)

	if err != nil {
		w.l.Errorw("failed to enqueue data", "object_path", uploadResult.Path, "error", err)
	} else {
		w.l.Debugw("finished enqueueing data", "object_path", uploadResult.Path)
	}
}

func compress(conf config.CompressionConfig, data []byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	compressWorker, err := compressor.NewWriter(&conf, buf)
	if err != nil {
		return nil, fmt.Errorf("error creating compressor: %w", err)
	}

	_, err = compressWorker.Write(data)
	if err != nil {
		return nil, fmt.Errorf("error writing compressed data into memory buffer: %w", err)
	}

	err = compressWorker.Close()
	if err != nil {
		return nil, fmt.Errorf("error writing (at the closing finish) compressed data into memory buffer: %w", err)
	}

	return buf.Bytes(), nil
}
