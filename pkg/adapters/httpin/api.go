package httpin

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jademcosta/jiboia/pkg/adapters/httpin/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
)

const APIComponentType = "api"
const apiVersion = "v1"

type API struct {
	mux  *chi.Mux
	log  *slog.Logger
	srv  *http.Server
	port int
}

func NewAPI(
	l *slog.Logger, conf config.Config, metricRegistry *prometheus.Registry, tracer trace.Tracer,
	appVersion string, flws []flow.Flow,
) *API {

	router := chi.NewRouter()
	logg := l.With(logger.ComponentKey, APIComponentType)

	sizeLimit, err := conf.API.PayloadSizeLimitInBytes()
	if err != nil {
		panic("payload size limit could not be extracted")
	}

	api := &API{
		mux:  router,
		log:  logg,
		srv:  &http.Server{Addr: fmt.Sprintf(":%d", conf.API.Port), Handler: router},
		port: conf.API.Port,
	}

	initializeMetrics(metricRegistry)
	registerDefaultMiddlewares(api, conf, sizeLimit, logg, metricRegistry, tracer)

	RegisterIngestingRoutes(api, apiVersion, flws)
	RegisterOperatinalRoutes(api, appVersion, metricRegistry)
	api.mux.Mount("/debug", middleware.Profiler())

	return api
}

func (api *API) ListenAndServe() error {
	api.log.Info(fmt.Sprintf("Starting HTTP server on port %d", api.port))
	err := api.srv.ListenAndServe()
	if err != nil {
		return fmt.Errorf("when serving HTTP: %w", err)
	}

	return nil
}

func (api *API) Shutdown() error {
	//TODO: allow the grace period to be configured
	shutdownCtx, shutdownCtxRelease := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCtxRelease()

	err := api.srv.Shutdown(shutdownCtx)
	return err
}

func registerDefaultMiddlewares(
	api *API,
	conf config.Config,
	sizeLimit int64,
	l *slog.Logger,
	metricRegistry *prometheus.Registry,
	tracer trace.Tracer,
) {

	//Middlewares on the top wrap the ones in the bottom
	api.mux.Use(httpmiddleware.NewLoggingMiddleware(l))
	if conf.O11y.TracingEnabled {
		api.mux.Use(httpmiddleware.NewTracingMiddleware(tracer))
	}
	api.mux.Use(httpmiddleware.NewMetricsMiddleware(metricRegistry))
	api.mux.Use(httpmiddleware.NewRecoverer(l))

	if sizeLimit > 0 {
		api.mux.Use(middleware.RequestSize(sizeLimit))
	}
}
