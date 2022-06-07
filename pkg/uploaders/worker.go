package uploaders

import (
	"context"
	"sync"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var ensureSingleMetricRegistration sync.Once

type ObjStorage interface {
	Upload(workU *domain.WorkUnit) (*domain.UploadResult, error)
}

type ExternalQueue interface {
	Enqueue(*domain.UploadResult) error
}

type Worker struct {
	l                    *zap.SugaredLogger
	workChan             chan *domain.WorkUnit
	storage              ObjStorage
	queue                ExternalQueue
	workVolunteeringChan chan chan *domain.WorkUnit
	ctx                  context.Context
	workInFlightGauge    *prometheus.GaugeVec
}

func NewWorker(
	l *zap.SugaredLogger,
	storage ObjStorage,
	extQueue ExternalQueue,
	workVolunteeringChan chan chan *domain.WorkUnit,
	metricRegistry *prometheus.Registry) *Worker {

	workInFlightGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "jiboia",
			Subsystem: "worker",
			Name:      "work_in_flight",
			Help:      "How many workers are performing work (vs being idle) right now.",
		},
		[]string{})

	ensureSingleMetricRegistration.Do(func() {
		metricRegistry.MustRegister(workInFlightGauge)
	})

	workChan := make(chan *domain.WorkUnit, 1)
	return &Worker{
		l:                    l,
		storage:              storage,
		queue:                extQueue,
		workVolunteeringChan: workVolunteeringChan,
		workChan:             workChan,
		workInFlightGauge:    workInFlightGauge,
	}
}

//Run should be called on a goroutine
func (w *Worker) Run(ctx context.Context) {
	w.ctx = ctx
	for {
		w.workVolunteeringChan <- w.workChan
		select {
		case workU := <-w.workChan:
			w.work(workU)
		case <-w.ctx.Done():
			return
		}

	}
}

func (w *Worker) work(workU *domain.WorkUnit) {
	w.workInFlightGauge.WithLabelValues().Inc()
	defer w.workInFlightGauge.WithLabelValues().Dec()

	uploadResult, err := w.storage.Upload(workU)

	if err != nil {
		w.l.Warn("failed to upload data", "prefix", workU.Prefix, "filename", workU.Filename, "error", err)
		return //FIXME: add tests for this return
	} else {
		w.l.Debug("finished uploading object", "prefix", workU.Prefix, "filename", workU.Filename)
	}

	err = w.queue.Enqueue(uploadResult)

	if err != nil {
		w.l.Warn("failed to enqueue data", "object_path", uploadResult.Path, "error", err)
	} else {
		w.l.Debug("finished enqueueing data", "object_path", uploadResult.Path)
	}
}
