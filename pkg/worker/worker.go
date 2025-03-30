package worker

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jademcosta/jiboia/pkg/compression"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
)

const SmallestAllowedCompressorWriter = 512

type ObjStorage interface {
	Upload(workU *domain.WorkUnit) (*domain.UploadResult, error)
}

type ExternalQueue interface {
	Enqueue(*domain.MessageContext) error
}

type Worker struct {
	l                   *slog.Logger
	storage             ObjStorage
	queue               ExternalQueue
	incomingWorkChan    <-chan *domain.WorkUnit
	flowName            string
	compressionConf     config.CompressionConfig
	currentTimeProvider func() time.Time
	doneChan            chan struct{}
}

func NewWorker(
	flowName string, l *slog.Logger, storage ObjStorage, extQueue ExternalQueue,
	incomingWorkChan <-chan *domain.WorkUnit, metricRegistry *prometheus.Registry,
	compressionConf config.CompressionConfig, currentTimeProvider func() time.Time,
) *Worker {

	initializeMetrics(metricRegistry)

	return &Worker{
		l:                   l.With(logger.ComponentKey, "worker"),
		storage:             storage,
		queue:               extQueue,
		incomingWorkChan:    incomingWorkChan,
		flowName:            flowName,
		compressionConf:     compressionConf,
		currentTimeProvider: currentTimeProvider,
	}
}

// Run should be called on a goroutine
func (w *Worker) Run(ctx context.Context) {
	w.doneChan = make(chan struct{})
	defer close(w.doneChan)

	for {
		select {
		case <-ctx.Done():
			w.drainWorkChan()
			return
		case workU := <-w.incomingWorkChan:
			if workU != nil {
				w.work(workU)
			}
		}
	}
}

func (w *Worker) Done() <-chan struct{} {
	return w.doneChan
}

func (w *Worker) work(workU *domain.WorkUnit) {
	incWorkInFlight(w.flowName)
	defer decWorkInFlight(w.flowName)

	compressedData, err := compress(w.compressionConf, workU.Data)
	if err != nil {
		w.l.Error("error compressing data", "prefix", workU.Prefix, "filename", workU.Filename, "error", err)
		return
	}

	workU.Data = compressedData
	uploadResult, err := w.storage.Upload(workU)
	if err != nil {
		w.l.Error("failed to upload object", "prefix", workU.Prefix, "filename", workU.Filename, "error", err)
		return
	}
	w.l.Debug("finished uploading object", "prefix", workU.Prefix, "filename", workU.Filename)

	uploadedAt := w.currentTimeProvider()

	msgContext := &domain.MessageContext{
		Bucket:          uploadResult.Bucket,
		Region:          uploadResult.Region,
		Path:            uploadResult.Path,
		URL:             uploadResult.URL,
		SizeInBytes:     uploadResult.SizeInBytes,
		CompressionType: w.compressionConf.Type,
		SavedAt:         uploadedAt.Unix(),
	}
	err = w.queue.Enqueue(msgContext)

	if err != nil {
		w.l.Error("failed to enqueue data", "object_path", uploadResult.Path, "error", err)
	} else {
		w.l.Debug("finished enqueueing data", "object_path", uploadResult.Path)
	}
}

func (w *Worker) drainWorkChan() {
	for {
		workU, moreWork := <-w.incomingWorkChan
		if !moreWork {
			break
		}

		w.work(workU)
	}
}

func compress(conf config.CompressionConfig, data []byte) ([]byte, error) {
	startTime := time.Now()
	buf := newCompressionResultBuffer(conf, len(data))
	compressWorker, err := compression.NewWriter(&conf, buf)
	if err != nil {
		return nil, fmt.Errorf("error creating compressor: %w", err)
	}

	_, err = compressWorker.Write(data)
	if err != nil {
		return nil, fmt.Errorf("error writing compressed data into memory buffer: %w", err)
	}

	err = compressWorker.Flush()
	if err != nil {
		return nil, fmt.Errorf("error writing (at the flush command) compressed data into memory buffer: %w", err)
	}

	err = compressWorker.Close()
	if err != nil {
		return nil, fmt.Errorf("error writing (at the closing finish) compressed data into memory buffer: %w", err)
	}

	compressedData := buf.Bytes()
	reportCompressionRatio(conf.Type, float64(len(compressedData))/float64(len(data)))
	reportCompressionDuration(conf.Type, time.Since(startTime))

	return compressedData, nil
}

func newCompressionResultBuffer(conf config.CompressionConfig, originalDataSize int) *bytes.Buffer {
	buf := &bytes.Buffer{}
	if conf.Type != "" {
		relativeSize := (originalDataSize * conf.PreallocSlicePercentage) / 100
		if relativeSize < SmallestAllowedCompressorWriter {
			relativeSize = SmallestAllowedCompressorWriter
		}
		buf.Grow(relativeSize)
	}

	return buf
}
