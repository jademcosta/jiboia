package uploaders

import (
	"context"
	"sync"

	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

var ensureSingleMetricRegistration sync.Once
var workInFlightGauge *prometheus.GaugeVec

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
	flowName             string
}

func NewWorker(
	flowName string,
	l *zap.SugaredLogger,
	storage ObjStorage,
	extQueue ExternalQueue,
	workVolunteeringChan chan chan *domain.WorkUnit,
	metricRegistry *prometheus.Registry) *Worker {

	ensureSingleMetricRegistration.Do(func() {
		workInFlightGauge = prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: "jiboia",
				Subsystem: "worker",
				Name:      "work_in_flight",
				Help:      "How many workers are performing work (vs being idle) right now.",
			},
			[]string{"flow"})

		metricRegistry.MustRegister(workInFlightGauge)
	})

	workChan := make(chan *domain.WorkUnit, 1)
	return &Worker{
		l:                    l.With(logger.COMPONENT_KEY, "worker"),
		storage:              storage,
		queue:                extQueue,
		workVolunteeringChan: workVolunteeringChan,
		workChan:             workChan,
		flowName:             flowName,
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

	uploadResult, err := w.storage.Upload(workU)

	if err != nil {
		w.l.Errorw("failed to upload object", "prefix", workU.Prefix, "filename", workU.Filename, "error", err)
		return
	} else {
		w.l.Debugw("finished uploading object", "prefix", workU.Prefix, "filename", workU.Filename)
	}

	err = w.queue.Enqueue(uploadResult)

	if err != nil {
		w.l.Errorw("failed to enqueue data", "object_path", uploadResult.Path, "error", err)
	} else {
		w.l.Debugw("finished enqueueing data", "object_path", uploadResult.Path)
	}
}
