package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"syscall"

	"github.com/jademcosta/jiboia/pkg/accumulators/non_blocking_bucket"
	"github.com/jademcosta/jiboia/pkg/adapters/external_queue"
	"github.com/jademcosta/jiboia/pkg/adapters/http_in"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/datetimeprovider"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"github.com/jademcosta/jiboia/pkg/uploaders/filepather"
	"github.com/jademcosta/jiboia/pkg/uploaders/nonblocking_uploader"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/zap"
)

type App struct {
	conf     *config.Config
	logger   *zap.SugaredLogger
	ctx      context.Context
	stopFunc context.CancelFunc
	flows    []*flow.Flow
}

func New(c *config.Config, logger *zap.SugaredLogger) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		conf:     c,
		logger:   logger,
		ctx:      ctx,
		stopFunc: cancel,
		flows:    make([]*flow.Flow, 0),
	}
}

func (a *App) Start() {
	metricRegistry := prometheus.NewRegistry()
	registerDefaultMetrics(metricRegistry)
	var uploaderShutdownWG sync.WaitGroup

	for _, flowConf := range a.conf.Flows {
		f := createFlow(a.logger, metricRegistry, flowConf)
		a.flows = append(a.flows, f)
	}

	api := http_in.New(a.logger, a.conf, metricRegistry, a.flows)

	//The shutdown of rungroup seems to be executed from a single goroutine. Meaning that if a
	//waitgroup is added on some interrupt function, it might hang forever.
	var g run.Group

	a.setupShutdownRelatedActors(&g)

	g.Add(
		func() error {
			uploaderShutdownWG.Add(1)
			err := api.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.logger.Errorw("api listening and serving failed", "error", err)
			}
			uploaderShutdownWG.Done()
			return err
		},
		func(error) {
			a.logger.Info("shutting down api")
			//TODO: Improve shutdown? Or oklog already takes care of it for us?
			//https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
			if err := api.Shutdown(); err != nil {
				a.logger.Errorw("api shutdown failed", "error", err)
			}
		},
	)

	for _, flow := range a.flows {

		if flow.Accumulator != nil {
			uploaderShutdownWG.Add(1)
			accumulatorContext, accumulatorCancel := context.WithCancel(context.Background())

			g.Add(
				func() error {
					flow.Accumulator.Run(accumulatorContext)
					uploaderShutdownWG.Done()
					return nil
				},
				func(error) {
					accumulatorCancel()
				},
			)
		}

		uploaderContext, uploaderCancel := context.WithCancel(context.Background())

		g.Add(
			func() error {
				flow.Uploader.Run(uploaderContext)
				return nil
			},
			func(error) {
				uploaderShutdownWG.Wait()
				uploaderCancel()
			},
		)

		for _, worker := range flow.Workers {
			go worker.Run(context.Background()) //TODO: we need to make uploader completelly stop the workers, for safety
		}
	}

	err := g.Run()
	if err != nil {
		a.logger.Errorw("something went wrong when running the components", "error", err)
	}
	a.logger.Info("jiboia stopped")
}

func (a *App) setupShutdownRelatedActors(g *run.Group) {
	signalsCh := make(chan os.Signal, 2)
	signal.Notify(signalsCh, syscall.SIGINT, syscall.SIGTERM)

	g.Add(func() error {
		select {
		case s := <-signalsCh:
			a.logger.Infow("received signal, shutting down", "signal", s.String())
		case <-a.ctx.Done():
		}
		return nil
	}, func(error) {
		a.stopFunc()
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	})
}

func (a *App) stop() {
	a.logger.Debug("app stop called")
	a.stopFunc()
}

func registerDefaultMetrics(registry *prometheus.Registry) {
	registry.MustRegister(collectors.NewBuildInfoCollector())
	// TODO: registry.MustRegister(collectors.NewProcessCollector())
	registry.MustRegister(collectors.NewGoCollector(
		collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}),
	))
}

func createObjStorage(l *zap.SugaredLogger, metricRegistry *prometheus.Registry, c config.ObjectStorage) uploaders.ObjStorage {
	objStorage, err := objstorage.New(l, metricRegistry, &c)
	if err != nil {
		l.Panic("error creating object storage", "error", err)
	}

	return objStorage
}

func createExternalQueue(l *zap.SugaredLogger, metricRegistry *prometheus.Registry, c config.ExternalQueue) uploaders.ExternalQueue {
	externalQueue, err := external_queue.New(l, metricRegistry, &c)
	if err != nil {
		l.Panic("error creating external queue", "error", err)
	}

	return externalQueue
}

func createFlow(logger *zap.SugaredLogger, metricRegistry *prometheus.Registry, flowConf config.FlowConfig) *flow.Flow {
	externalQueue := createExternalQueue(logger, metricRegistry, flowConf.ExternalQueue)
	objStorage := createObjStorage(logger, metricRegistry, flowConf.ObjectStorage)

	uploader := nonblocking_uploader.New(
		logger,
		flowConf.MaxConcurrentUploads,
		flowConf.QueueMaxSize,
		domain.NewObservableDataDropper(logger, metricRegistry, "uploader"),
		filepather.New(datetimeprovider.New(), flowConf.PathPrefixCount),
		metricRegistry)

	var accumulator flow.RunnableFlow
	if flowConf.Accumulator.SizeInBytes > 0 {
		accumulator = non_blocking_bucket.New(
			logger,
			flowConf.Accumulator.SizeInBytes,
			[]byte(flowConf.Accumulator.Separator),
			flowConf.Accumulator.QueueCapacity,
			domain.NewObservableDataDropper(logger, metricRegistry, "accumulator"),
			uploader,
			metricRegistry)
	}

	workers := make([]flow.Runnable, 0, flowConf.MaxConcurrentUploads)
	for i := 0; i < flowConf.MaxConcurrentUploads; i++ {
		workers = append(workers, uploaders.NewWorker(logger, objStorage, externalQueue, uploader.WorkersReady, metricRegistry))
	}

	return flow.New(objStorage, externalQueue, uploader, accumulator, workers)
}
