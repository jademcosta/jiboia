package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/jademcosta/jiboia/pkg/accumulator"
	"github.com/jademcosta/jiboia/pkg/adapters/externalqueue"
	"github.com/jademcosta/jiboia/pkg/adapters/httpin"
	"github.com/jademcosta/jiboia/pkg/adapters/objstorage"
	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/datetimeprovider"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/jademcosta/jiboia/pkg/o11y/tracing"
	"github.com/jademcosta/jiboia/pkg/uploader"
	"github.com/jademcosta/jiboia/pkg/uploader/filepather"
	"github.com/jademcosta/jiboia/pkg/worker"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/sony/gobreaker"
)

type App struct {
	conf         *config.Config
	logger       *slog.Logger
	ctx          context.Context
	stopFunc     context.CancelFunc
	shutdownDone chan struct{}
}

func New(c *config.Config, logger *slog.Logger) *App {
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

	tracer, shutdownFunc := tracing.NewTracer(a.conf.O11y.Tracing)
	defer shutdownTracer(shutdownFunc, a.logger)

	flows := createFlows(a.logger, metricRegistry, a.conf.Flows)
	api := httpin.NewAPI(a.logger, *a.conf, metricRegistry, tracer, a.conf.Version, flows)

	//The shutdown of rungroup seems to be executed from a single goroutine. Meaning that if a
	//waitgroup is added on some interrupt function, it might hang forever.
	var g run.Group

	a.addShutdownRelatedActors(&g)

	g.Add(func() error {
		err := api.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("api listening and serving failed", "error", err)
		}

		return err
	}, func(_ error) {
		a.logger.Info("shutting down api")
		if err := api.Shutdown(); err != nil {
			a.logger.Error("api shutdown failed", "error", err)
		}
	})

	for _, flw := range flows {
		flowCopy := flw
		addFlowActorToRunGroup(&g, &flowCopy)
	}

	err := g.Run()
	if err != nil {
		a.logger.Error("something went wrong when running the components", "error", err)
	}
	a.logger.Info("jiboia stopped")
}

func (a *App) addShutdownRelatedActors(g *run.Group) {
	signalsCh := make(chan os.Signal, 2)
	signal.Notify(signalsCh, syscall.SIGINT, syscall.SIGTERM)

	g.Add(func() error {
		select {
		case s := <-signalsCh:
			a.logger.Info("received signal, shutting down", "signal", s)
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

func addFlowActorToRunGroup(g *run.Group, flw *flow.Flow) {
	if flw.Accumulator != nil {
		accumulatorContext, accumulatorCancel := context.WithCancel(context.Background())

		g.Add(func() error {
			flw.Accumulator.Run(accumulatorContext)
			return nil
		}, func(_ error) {
			accumulatorCancel()
			<-flw.Accumulator.Done()
		})
	}

	uploaderContext, uploaderCancel := context.WithCancel(context.Background())
	g.Add(func() error {
		flw.Uploader.Run(uploaderContext)
		return nil
	}, func(_ error) {
		uploaderCancel()
		<-flw.Uploader.Done()
	})

	workersContext, workersCancel := context.WithCancel(context.Background()) //nolint: govet
	for _, worker := range flw.UploadWorkers {
		workerCopy := worker
		g.Add(func() error {
			workerCopy.Run(workersContext)
			return nil
		}, func(_ error) {
			workersCancel()
			<-workerCopy.Done()
		})
	}
} //nolint: govet

func registerDefaultMetrics(registry *prometheus.Registry) {
	registry.MustRegister(
		collectors.NewBuildInfoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(
			collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}),
		),
	)
}

func createObjStorage(
	l *slog.Logger, c config.ObjectStorageConfig, metricRegistry *prometheus.Registry,
	flowName string,
) worker.ObjStorage {
	objStorage, err := objstorage.New(l, metricRegistry, flowName, &c)
	if err != nil {
		l.Error("error creating object storage", "error", err)
		panic("error creating object storage")
	}

	return objStorage
}

func createExternalQueue(
	l *slog.Logger, c config.ExternalQueueConfig, metricRegistry *prometheus.Registry,
	flowName string,
) worker.ExternalQueue {
	externalQueue, err := externalqueue.New(l, metricRegistry, flowName, &c)
	if err != nil {
		l.Error("error creating external queue", "error", err)
		panic("error creating external queue")
	}

	return externalQueue
}

func createAccumulator(
	flowName string, logger *slog.Logger, c config.AccumulatorConfig,
	registry *prometheus.Registry, uploader domain.DataFlow,
) *accumulator.AccumulatorBySize {
	cb := createCircuitBreaker(registry, logger, flowName, c.CircuitBreaker)

	sizeAsBytes, err := c.SizeAsBytes()
	if err != nil {
		panic(fmt.Errorf("error parsing accumulator size: %w", err))
	}

	return accumulator.NewAccumulatorBySize(
		flowName,
		logger,
		int(sizeAsBytes),
		[]byte(c.Separator),
		c.QueueCapacity,
		uploader,
		cb,
		registry)
}

func createFlows(
	llog *slog.Logger, metricRegistry *prometheus.Registry,
	confs []config.FlowConfig,
) []flow.Flow {

	flows := make([]flow.Flow, 0, len(confs))

	for _, conf := range confs {
		flowConf := conf
		localLogger := llog.With(logger.FlowKey, flowConf.Name)
		externalQueue := createExternalQueue(localLogger, flowConf.ExternalQueue, metricRegistry, flowConf.Name)
		objStorage := createObjStorage(localLogger, flowConf.ObjectStorage, metricRegistry, flowConf.Name)
		workersIngestionQueue := make(chan *domain.WorkUnit, flowConf.MaxConcurrentUploads)

		uploader := uploader.New(
			flowConf.Name,
			localLogger,
			flowConf.MaxConcurrentUploads,
			flowConf.QueueMaxSize,
			filepather.New(datetimeprovider.New(), flowConf.PathPrefixCount, flowConf.Compression.Type),
			metricRegistry,
			workersIngestionQueue,
		)

		initialDecompressionBufferSize, err := flowConf.Ingestion.Decompression.InitialBufferSizeAsBytes()
		if err != nil {
			panic(fmt.Errorf("error parsing initial buffer size: %w", err))
		}

		f := flow.Flow{
			Name:                                flowConf.Name,
			ObjStorage:                          objStorage,
			ExternalQueue:                       externalQueue,
			Uploader:                            uploader,
			UploadWorkers:                       make([]flow.Runnable, 0, flowConf.MaxConcurrentUploads),
			Token:                               flowConf.Ingestion.Token,
			DecompressionAlgorithms:             flowConf.Ingestion.Decompression.ActiveDecompressions,
			DecompressionMaxConcurrency:         flowConf.Ingestion.Decompression.MaxConcurrency,
			DecompressionInitialBufferSizeBytes: int(initialDecompressionBufferSize),
			CircuitBreaker:                      createTwoStepCircuitBreaker(metricRegistry, llog, flowConf.Name, flowConf.Ingestion.CircuitBreaker),
		}

		accSizeAsBytes, err := flowConf.Accumulator.SizeAsBytes()
		if err != nil {
			panic(fmt.Errorf("error parsing accumulator size: %w", err))
		}
		hasAccumulatorDeclared := accSizeAsBytes > 0 //TODO: this is something that will need to be improved, as it is error prone
		if hasAccumulatorDeclared {
			f.Accumulator = createAccumulator(flowConf.Name, localLogger, flowConf.Accumulator,
				metricRegistry, uploader)
		}

		for i := 0; i < flowConf.MaxConcurrentUploads; i++ {
			worker := worker.NewWorker(
				flowConf.Name, localLogger, objStorage, externalQueue, workersIngestionQueue,
				metricRegistry, flowConf.Compression, time.Now,
			)
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

func createTwoStepCircuitBreaker(
	registry *prometheus.Registry, logg *slog.Logger, flowName string,
	cbConf config.CircuitBreakerConfig,
) circuitbreaker.TwoStepCircuitBreaker {

	if cbConf.Disable {
		return circuitbreaker.NewDummyTwoStepCircuitBreaker()
	}

	name := fmt.Sprintf("%s_ingestion_cb", flowName)
	cbO11y := circuitbreaker.NewCBObservability(registry, logg, name, flowName)

	return gobreaker.NewTwoStepCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1, //FIXME: magic number. This should be extracted into a const
		Timeout:     cbConf.OpenIntervalAsDuration(),
		ReadyToTrip: func(_ gobreaker.Counts) bool {
			return true
		},
		OnStateChange: func(_ string, _, to gobreaker.State) {
			if to == gobreaker.StateOpen {
				cbO11y.SetCBOpen()
			} else {
				cbO11y.SetCBClosed()
			}
		},
	})
}

func createCircuitBreaker(
	registry *prometheus.Registry, llog *slog.Logger, flowName string,
	cbConf config.CircuitBreakerConfig,
) circuitbreaker.CircuitBreaker {

	if cbConf.Disable {
		return circuitbreaker.NewDummyCircuitBreaker()
	}

	name := fmt.Sprintf("%s_accumulator_cb", flowName)
	cbO11y := circuitbreaker.NewCBObservability(registry, llog, name, flowName)

	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: 1, //FIXME: magic number. This should be extracted into a const
		Timeout:     cbConf.OpenIntervalAsDuration(),
		ReadyToTrip: func(_ gobreaker.Counts) bool {
			return true
		},
		OnStateChange: func(_ string, _, to gobreaker.State) {
			if to == gobreaker.StateOpen {
				cbO11y.SetCBOpen()
			} else {
				cbO11y.SetCBClosed()
			}
		},
	})

}

func shutdownTracer(shutdownFunc func(context.Context) error, logg *slog.Logger) {
	ctxForShutdown, cancelFunc := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelFunc()
	err := shutdownFunc(ctxForShutdown)
	if err != nil {
		logg.Error("when shutting down tracing", "error", err)
	}
}
