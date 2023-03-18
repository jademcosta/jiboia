package app

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/jademcosta/jiboia/pkg/accumulators/non_blocking_bucket"
	"github.com/jademcosta/jiboia/pkg/adapters/external_queue"
	"github.com/jademcosta/jiboia/pkg/adapters/http_in"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/datetimeprovider"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"github.com/jademcosta/jiboia/pkg/uploaders/filepather"
	"github.com/jademcosta/jiboia/pkg/uploaders/nonblocking_uploader"
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

func (a *App) stop() <-chan struct{} {
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

func createObjStorage(l *zap.SugaredLogger, c config.ObjectStorage, metricRegistry *prometheus.Registry) uploaders.ObjStorage {
	objStorage, err := objstorage.New(l, metricRegistry, &c)
	if err != nil {
		l.Panic("error creating object storage", "error", err)
	}

	return objStorage
}

func createExternalQueue(l *zap.SugaredLogger, c config.ExternalQueue, metricRegistry *prometheus.Registry) uploaders.ExternalQueue {
	externalQueue, err := external_queue.New(l, metricRegistry, &c)
	if err != nil {
		l.Panic("error creating external queue", "error", err)
	}

	return externalQueue
}

func createAccumulator(flowName string, l *zap.SugaredLogger, c config.Accumulator, registry *prometheus.Registry, uploader domain.DataFlow) *non_blocking_bucket.BucketAccumulator {
	return non_blocking_bucket.New(
		flowName,
		l,
		c.SizeInBytes,
		[]byte(c.Separator),
		c.QueueCapacity,
		domain.NewObservableDataDropper(l, registry, "accumulator"),
		uploader, registry)
}

func createFlows(llog *zap.SugaredLogger, metricRegistry *prometheus.Registry,
	confs []config.FlowConfig) []flow.Flow {

	flows := make([]flow.Flow, 0, len(confs))

	for _, conf := range confs {
		localLogger := llog.With(logger.FLOW_KEY, conf.Name)
		externalQueue := createExternalQueue(localLogger, conf.ExternalQueue, metricRegistry)
		objStorage := createObjStorage(localLogger, conf.ObjectStorage, metricRegistry)

		uploader := nonblocking_uploader.New(
			conf.Name,
			localLogger,
			conf.MaxConcurrentUploads,
			conf.QueueMaxSize,
			domain.NewObservableDataDropper(localLogger, metricRegistry, "uploader"),
			filepather.New(datetimeprovider.New(), conf.PathPrefixCount),
			metricRegistry)

		f := flow.Flow{
			Name:          conf.Name,
			ObjStorage:    objStorage,
			ExternalQueue: externalQueue,
			Uploader:      uploader,
			UploadWorkers: make([]flow.Runnable, 0, conf.MaxConcurrentUploads),
		}

		hasAccumulatorDeclared := conf.Accumulator.SizeInBytes > 0
		if hasAccumulatorDeclared {
			f.Accumulator = createAccumulator(conf.Name, localLogger, conf.Accumulator, metricRegistry, uploader)
		}

		for i := 0; i < conf.MaxConcurrentUploads; i++ {
			worker := uploaders.NewWorker(conf.Name, localLogger, objStorage, externalQueue, uploader.WorkersReady, metricRegistry)
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
