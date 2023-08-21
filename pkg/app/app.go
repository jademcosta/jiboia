package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/jademcosta/jiboia/pkg/accumulator"
	"github.com/jademcosta/jiboia/pkg/adapters/external_queue"
	"github.com/jademcosta/jiboia/pkg/adapters/http_in"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage"
	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/datetimeprovider"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/uploader"
	"github.com/jademcosta/jiboia/pkg/uploader/filepather"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/zap"
)

type App struct {
	conf         *config.Config
	logger       *zap.SugaredLogger
	ctx          context.Context
	stopFunc     context.CancelFunc
	shutdownDone chan struct{}
}

func New(c *config.Config, logger *zap.SugaredLogger) *App {
	ctx, cancel := context.WithCancel(context.Background())

	return &App{
		conf:         c,
		logger:       logger,
		ctx:          ctx,
		stopFunc:     cancel,
		shutdownDone: make(chan struct{}),
	}
}

func (a *App) Start() {
	defer close(a.shutdownDone)
	metricRegistry := prometheus.NewRegistry()
	registerDefaultMetrics(metricRegistry)

	flows := createFlows(a.logger, metricRegistry, a.conf.Flows)

	apiShutdownDone := make(chan struct{})

	api := http_in.New(a.logger, a.conf.Api, metricRegistry, a.conf.Version, flows)

	//The shutdown of rungroup seems to be executed from a single goroutine. Meaning that if a
	//waitgroup is added on some interrupt function, it might hang forever.
	var g run.Group

	a.addShutdownRelatedActors(&g)

	g.Add(
		func() error {
			defer close(apiShutdownDone)
			err := api.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.logger.Errorw("api listening and serving failed", "error", err)
			}

			return err
		},
		func(error) {
			a.logger.Info("shutting down api")
			//TODO: Improve shutdown? Or oklog already takes care of it for us?
			//https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
			// apiCancel() // FIXME: I believe we might not need this
			if err := api.Shutdown(); err != nil {
				a.logger.Errorw("api shutdown failed", "error", err)
			}
		},
	)

	for _, flw := range flows {
		flowCopy := flw
		addFlowActorToRunGroup(&g, apiShutdownDone, &flowCopy)
	}

	err := g.Run()
	if err != nil {
		a.logger.Errorw("something went wrong when running the components", "error", err)
	}
	a.logger.Info("jiboia stopped")
}

func (a *App) addShutdownRelatedActors(g *run.Group) {
	signalsCh := make(chan os.Signal, 2)
	signal.Notify(signalsCh, syscall.SIGINT, syscall.SIGTERM)

	g.Add(func() error {
		select {
		case s := <-signalsCh:
			a.logger.Infow("received signal, shutting down", "signal", s)
		case <-a.ctx.Done():
		}
		return nil
	}, func(error) {
		a.stopFunc()
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	})
}

func (a *App) Stop() <-chan struct{} {
	a.logger.Debug("app stop called")
	a.stopFunc()
	return a.shutdownDone
}

func addFlowActorToRunGroup(g *run.Group, apiShutdownDone <-chan struct{}, flw *flow.Flow) {

	accumulatorShutdownDone := make(chan struct{})
	uploaderShutdownDone := make(chan struct{})

	if flw.Accumulator != nil {
		accumulatorContext, accumulatorCancel := context.WithCancel(context.Background())

		g.Add(
			func() error {
				defer close(accumulatorShutdownDone)
				flw.Accumulator.Run(accumulatorContext)
				return nil
			},
			func(error) {
				<-apiShutdownDone
				accumulatorCancel()
			},
		)
	} else {
		close(accumulatorShutdownDone)
	}

	uploaderContext, uploaderCancel := context.WithCancel(context.Background())
	g.Add(
		func() error {
			defer close(uploaderShutdownDone)
			flw.Uploader.Run(uploaderContext)
			return nil
		},
		func(error) {
			<-apiShutdownDone
			<-accumulatorShutdownDone
			uploaderCancel()
		},
	)

	workersContext, workersCancel := context.WithCancel(context.Background()) //nolint:govet
	for _, worker := range flw.UploadWorkers {
		workerCopy := worker
		g.Add(
			func() error {
				workerCopy.Run(workersContext)
				return nil
			},
			func(error) {
				<-uploaderShutdownDone
				workersCancel()
			},
		)
	}
} //nolint:govet

func registerDefaultMetrics(registry *prometheus.Registry) {
	registry.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}),
		),
	)
}

func createObjStorage(l *zap.SugaredLogger, c config.ObjectStorage, metricRegistry *prometheus.Registry, flowName string) worker.ObjStorage {
	objStorage, err := objstorage.New(l, metricRegistry, flowName, &c)
	if err != nil {
		l.Panicw("error creating object storage", "error", err)
	}

	return objStorage
}

func createExternalQueue(l *zap.SugaredLogger, c config.ExternalQueue, metricRegistry *prometheus.Registry, flowName string) worker.ExternalQueue {
	externalQueue, err := external_queue.New(l, metricRegistry, flowName, &c)
	if err != nil {
		l.Panicw("error creating external queue", "error", err)
	}

	return externalQueue
}

func createAccumulator(flowName string, logger *zap.SugaredLogger, c config.Accumulator, registry *prometheus.Registry, uploader domain.DataFlow) *accumulator.BucketAccumulator {
	cb, err := circuitbreaker.FromConfig(logger, registry, c.CircuitBreaker, accumulator.COMPONENT_NAME, flowName)
	if err != nil {
		logger.Panicw("error on accumulator creation", "error", err)
	}

	return accumulator.New(
		flowName,
		logger,
		c.SizeInBytes,
		[]byte(c.Separator),
		c.QueueCapacity,
		domain.NewObservableDataDropper(logger, registry, accumulator.COMPONENT_NAME),
		uploader,
		cb,
		registry)
}

func createFlows(llog *zap.SugaredLogger, metricRegistry *prometheus.Registry,
	confs []config.FlowConfig) []flow.Flow {

	flows := make([]flow.Flow, 0, len(confs))

	for _, conf := range confs {
		flowConf := conf
		localLogger := llog.With(logger.FLOW_KEY, flowConf.Name)
		externalQueue := createExternalQueue(localLogger, flowConf.ExternalQueue, metricRegistry, flowConf.Name)
		objStorage := createObjStorage(localLogger, flowConf.ObjectStorage, metricRegistry, flowConf.Name)

		uploader := uploader.New(
			flowConf.Name,
			localLogger,
			flowConf.MaxConcurrentUploads,
			flowConf.QueueMaxSize,
			domain.NewObservableDataDropper(localLogger, metricRegistry, "uploader"),
			filepather.New(datetimeprovider.New(), flowConf.PathPrefixCount, flowConf.Compression.Type),
			metricRegistry)

		f := flow.Flow{
			Name:          flowConf.Name,
			ObjStorage:    objStorage,
			ExternalQueue: externalQueue,
			Uploader:      uploader,
			UploadWorkers: make([]flow.Runnable, 0, flowConf.MaxConcurrentUploads),
			Token:         flowConf.Ingestion.Token,
		}

		hasAccumulatorDeclared := flowConf.Accumulator.SizeInBytes > 0 //TODO: this is something that will need to be improved once config is localized inside packages
		if hasAccumulatorDeclared {
			f.Accumulator = createAccumulator(flowConf.Name, localLogger, flowConf.Accumulator, metricRegistry, uploader)
		}

		for i := 0; i < flowConf.MaxConcurrentUploads; i++ {
			worker := worker.NewWorker(flowConf.Name, localLogger, objStorage, externalQueue, uploader.WorkersReady, metricRegistry, flowConf.Compression)
			f.UploadWorkers = append(f.UploadWorkers, worker)
		}

		f.Entrypoint = uploader
		if f.Accumulator != nil {
			f.Entrypoint = f.Accumulator
		}

		flows = append(flows, f)
	}

	return flows
}
