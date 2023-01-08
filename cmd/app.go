package main

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
	"github.com/jademcosta/jiboia/pkg/uploaders"
	"github.com/jademcosta/jiboia/pkg/uploaders/filepather"
	"github.com/jademcosta/jiboia/pkg/uploaders/nonblocking_uploader"
	"github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.uber.org/zap"
)

type app struct {
	conf     *config.Config
	log      *zap.SugaredLogger
	ctx      context.Context
	stopFunc context.CancelFunc
}

func New(c *config.Config, l *zap.SugaredLogger) *app {
	ctx, cancel := context.WithCancel(context.Background())

	return &app{
		conf:     c,
		log:      l,
		ctx:      ctx,
		stopFunc: cancel,
	}
}

func (a *app) start() {
	metricRegistry := prometheus.NewRegistry()
	registerDefaultMetrics(metricRegistry)

	externalQueue := createExternalQueue(a.log, a.conf, metricRegistry)
	objStorage := createObjStorage(a.log, a.conf, metricRegistry)
	var uploaderWG sync.WaitGroup

	uploader := nonblocking_uploader.New(
		a.log,
		a.conf.Flow.MaxConcurrentUploads,
		a.conf.Flow.QueueMaxSize,
		domain.NewObservableDataDropper(a.log, metricRegistry, "uploader"),
		filepather.New(datetimeprovider.New()),
		metricRegistry)

	uploaderContext, uploaderCancel := context.WithCancel(context.Background())

	var flowEntrypoint domain.DataFlow
	flowEntrypoint = uploader
	accumulator := createAccumulator(a.log, &a.conf.Flow.Accumulator, metricRegistry, uploader)
	if accumulator != nil {
		flowEntrypoint = accumulator
	}

	api := http_in.New(a.log, a.conf, metricRegistry, flowEntrypoint)

	//The shutdown of rungroup seems to be executed from a single goroutine. Meaning that if a
	//waitgroup is added on some interrupt function, it might hang forever.
	var g run.Group

	a.addShutdownRelatedActors(&g)

	g.Add(
		func() error {
			uploaderWG.Add(1)
			err := api.ListenAndServe()
			if err != nil && !errors.Is(err, http.ErrServerClosed) {
				a.log.Errorw("api listening and serving failed", "error", err)
			}
			uploaderWG.Done()
			return err
		},
		func(error) {
			a.log.Info("shutting down api")
			//TODO: Improve shutdown? Or oklog already takes care of it for us?
			//https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
			// apiCancel() // FIXME: I believe we might not need this
			if err := api.Shutdown(); err != nil {
				a.log.Errorw("api shutdown failed", "error", err)
			}
		},
	)

	if accumulator != nil {
		uploaderWG.Add(1)
		flowEntrypoint = accumulator
		accumulatorContext, accumulatorCancel := context.WithCancel(context.Background())

		g.Add(
			func() error {
				accumulator.Run(accumulatorContext)
				uploaderWG.Done()
				return nil
			},
			func(error) {
				accumulatorCancel()
			},
		)
	}

	g.Add(
		func() error {
			uploader.Run(uploaderContext)
			return nil
		},
		func(error) {
			uploaderWG.Wait()
			uploaderCancel()
		},
	)

	for i := 0; i < a.conf.Flow.MaxConcurrentUploads; i++ {
		worker := uploaders.NewWorker(a.log, objStorage, externalQueue, uploader.WorkersReady, metricRegistry)
		go worker.Run(context.Background()) //TODO: we need to make uploader completelly stop the workers, for safety
	}

	err := g.Run()
	if err != nil {
		a.log.Errorw("something went wrong when running the components", "error", err)
	}
	a.log.Info("jiboia stopped")
}

func (a *app) addShutdownRelatedActors(g *run.Group) {
	signalsCh := make(chan os.Signal, 2)
	signal.Notify(signalsCh, syscall.SIGINT, syscall.SIGTERM)

	g.Add(func() error {
		select {
		case s := <-signalsCh:
			a.log.Infow("received signal, shutting down", "signal", s)
		case <-a.ctx.Done():
		}
		return nil
	}, func(error) {
		a.stopFunc()
		signal.Reset(syscall.SIGINT, syscall.SIGTERM)
	})
}

func (a *app) stop() {
	a.log.Debug("app stop called")
	a.stopFunc()
}

func createObjStorage(l *zap.SugaredLogger, c *config.Config, metricRegistry *prometheus.Registry) uploaders.ObjStorage {
	objStorage, err := objstorage.New(l, metricRegistry, &c.Flow.ObjectStorage)
	if err != nil {
		l.Panic("error creating object storage", "error", err)
	}

	return objStorage
}

func createExternalQueue(l *zap.SugaredLogger, c *config.Config, metricRegistry *prometheus.Registry) uploaders.ExternalQueue {
	externalQueue, err := external_queue.New(l, metricRegistry, &c.Flow.ExternalQueue)
	if err != nil {
		l.Panic("error creating external queue", "error", err)
	}

	return externalQueue
}

func createAccumulator(l *zap.SugaredLogger, c *config.Accumulator, registry *prometheus.Registry, uploader domain.DataFlow) *non_blocking_bucket.BucketAccumulator {
	//TODO: use generic factory
	if c.SizeInBytes > 0 {
		return non_blocking_bucket.New(
			l,
			c.SizeInBytes,
			[]byte(c.Separator),
			c.QueueCapacity,
			domain.NewObservableDataDropper(l, registry, "accumulator"),
			uploader, registry)
	}
	return nil
}

func registerDefaultMetrics(registry *prometheus.Registry) {
	registry.MustRegister(collectors.NewBuildInfoCollector())
	// TODO: registry.MustRegister(collectors.NewProcessCollector())
	registry.MustRegister(collectors.NewGoCollector(
		collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}),
	))
}
