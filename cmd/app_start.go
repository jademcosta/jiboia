package main

import (
	"context"
	"regexp"

	"github.com/jademcosta/jiboia/pkg/accumulators/non_blocking_bucket"
	"github.com/jademcosta/jiboia/pkg/adapters/external_queue"
	"github.com/jademcosta/jiboia/pkg/adapters/external_queue/sqs"
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

func startApps(c *config.Config, l *zap.SugaredLogger) {

	var g run.Group

	metricRegistry := prometheus.NewRegistry()
	registerDefaultMetrics(metricRegistry)

	externalQueue := createExternalQueue(l, c, metricRegistry)
	objStorage := createObjStorage(l, c, metricRegistry)

	uploader := nonblocking_uploader.New(
		l,
		c.Flow.MaxConcurrentUploads,
		c.Flow.QueueMaxSize,
		domain.NewObservableDataDropper(l, metricRegistry, "uploader"),
		filepather.New(datetimeprovider.New()),
		metricRegistry)

	uploaderContext, uploaderCancel := context.WithCancel(context.Background())
	workersContext, workersCancel := context.WithCancel(context.Background())

	g.Add(
		func() error {
			uploader.Run(uploaderContext)
			return nil
		},
		func(error) {
			l.Info("shutting down uploader")
			uploaderCancel()
			workersCancel()
		},
	)

	for i := 0; i < c.Flow.MaxConcurrentUploads; i++ {
		worker := uploaders.NewWorker(l, objStorage, externalQueue, uploader.WorkersReady, metricRegistry)

		g.Add(
			func() error {
				worker.Run(workersContext)
				return nil
			},
			func(error) {
				workersCancel()
			},
		)
	}

	var apiEntrypoint domain.DataFlow

	accumulator := createAccumulator(l, &c.Flow.Accumulator, metricRegistry, uploader)
	if accumulator != nil {
		apiEntrypoint = accumulator
	} else {
		apiEntrypoint = uploader
	}

	if accumulator != nil {
		accumulatorContext, accumulatorCancel := context.WithCancel(context.Background())
		g.Add(
			func() error {
				accumulator.Run(accumulatorContext)
				return nil
			},
			func(error) {
				accumulatorCancel()
			},
		)
	}

	// apiContext, apiCancel := context.WithCancel(context.Background())
	api := http_in.New(l, c, metricRegistry, apiEntrypoint)

	g.Add(
		func() error {
			err := api.ListenAndServe()
			if err != nil {
				l.Error("api listening and serving failed", "error", err)
			}
			return err
		},
		func(error) {
			l.Info("shutting down api")
			//TODO: Improve shutdown? Or oklog already takes care of it for us?
			//https://stackoverflow.com/questions/39320025/how-to-stop-http-listenandserve
			// apiCancel() // FIXME: I believe we might not need this
			if err := api.Shutdown(); err != nil {
				l.Error("api shutdown failed", "error", err)
			}
		},
	)

	err := g.Run()
	if err != nil {
		l.Error("something went wrong", "error", err)
	}
	l.Info("jiboia exiting")

	//TODO: add actor that listen to termination signals
}

func createObjStorage(l *zap.SugaredLogger, c *config.Config, metricRegistry *prometheus.Registry) uploaders.ObjStorage {
	objStorage, err := objstorage.New(l, &c.Flow.ObjectStorage)
	if err != nil {
		l.Panic("error creating object storage", "error", err)
	}

	return objstorage.NewStorageWithMetrics(objStorage, metricRegistry)
}

func createExternalQueue(l *zap.SugaredLogger, c *config.Config, metricRegistry *prometheus.Registry) uploaders.ExternalQueue {
	externalQueue, err := sqs.New(l, &c.Flow.ExternalQueue.Config)
	//TODO: replace with generic factory
	if err != nil {
		l.Panic("error creating external queue", "error", err)
	}

	return external_queue.NewExternalQueueWithMetrics(externalQueue, metricRegistry)
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
	registry.MustRegister(collectors.NewGoCollector(
		collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: regexp.MustCompile("/.*")}),
	))
}
