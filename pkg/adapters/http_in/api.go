package http_in

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jademcosta/jiboia/pkg/adapters/http_in/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

const API_COMPONENT_TYPE = "api"

type Api struct {
	mux        *chi.Mux
	log        *zap.SugaredLogger
	srv        *http.Server
	port       int
	sizeMetric *prometheus.HistogramVec
}

// TODO: remove the full config dep
func New(l *zap.SugaredLogger, c *config.Config, metricRegistry *prometheus.Registry, flows []*flow.Flow) *Api {

	router := chi.NewRouter()

	api := &Api{
		mux:  router,
		log:  l.With(logger.COMPONENT_KEY, API_COMPONENT_TYPE),
		srv:  &http.Server{Addr: fmt.Sprintf(":%d", c.Api.Port), Handler: router},
		port: c.Api.Port,
	}

	registerDefaultMiddlewares(api, metricRegistry)

	api.sizeMetric = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:      "request_body_size_bytes",
			Subsystem: "http",
			Namespace: "jiboia",
			Help:      "The size in bytes of (received) request body",
			//TODO: make these buckets configurable
			Buckets: []float64{0, 1024, 524288, 1048576, 2621440, 5242880, 10485760, 52428800, 104857600},
			// 0, 1KB, 512KB, 1MB, 2.5MB, 5MB, 10MB, 50MB, 100MB
		},
		[]string{"path"},
	)

	metricRegistry.MustRegister(api.sizeMetric)

	RegisterIngestingRoutes(api, flows) //TODO: add middleware that will return syntax error in case a request comes with no body
	RegisterOperatinalRoutes(api, c, metricRegistry)
	api.mux.Mount("/debug", middleware.Profiler())

	return api
}

func (api *Api) ListenAndServe() error {
	api.log.Info(fmt.Sprintf("Starting HTTP server on port %d", api.port))
	return fmt.Errorf("on serving HTTP: %w", api.srv.ListenAndServe())
}

func (api *Api) Shutdown() error {
	//TODO: allow the grace period to be configured
	shutdownCtx, shutdownCtxRelease := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCtxRelease()

	err := api.srv.Shutdown(shutdownCtx)
	return err
}

func registerDefaultMiddlewares(api *Api, metricRegistry *prometheus.Registry) {
	//Middlewares on the top wrap the ones in the bottom
	api.mux.Use(httpmiddleware.NewLoggingMiddleware(api.log))
	api.mux.Use(httpmiddleware.NewMetricsMiddleware(metricRegistry))
	api.mux.Use(httpmiddleware.NewRecoverer(api.log))
}
