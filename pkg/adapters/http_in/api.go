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
	mux  *chi.Mux
	log  *zap.SugaredLogger
	srv  *http.Server
	port int
}

func New(l *zap.SugaredLogger, conf config.ApiConfig, metricRegistry *prometheus.Registry,
	appVersion string, flws []flow.Flow) *Api {

	router := chi.NewRouter()
	logg := l.With(logger.COMPONENT_KEY, API_COMPONENT_TYPE)

	sizeLimit, err := conf.PayloadSizeLimitInBytes()
	if err != nil {
		panic("payload size limit could not be extracted")
	}

	api := &Api{
		mux:  router,
		log:  logg,
		srv:  &http.Server{Addr: fmt.Sprintf(":%d", conf.Port), Handler: router},
		port: conf.Port,
	}

	initializeMetrics(metricRegistry)
	registerDefaultMiddlewares(api, sizeLimit, logg, metricRegistry)

	RegisterIngestingRoutes(api, sizeHist, flws)
	RegisterOperatinalRoutes(api, appVersion, metricRegistry)
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

func registerDefaultMiddlewares(
	api *Api,
	sizeLimit int,
	l *zap.SugaredLogger,
	metricRegistry *prometheus.Registry,
) {

	//Middlewares on the top wrap the ones in the bottom
	api.mux.Use(httpmiddleware.NewLoggingMiddleware(l))
	api.mux.Use(httpmiddleware.NewMetricsMiddleware(metricRegistry))
	api.mux.Use(httpmiddleware.NewRecoverer(l))

	if sizeLimit > 0 {
		api.mux.Use(middleware.RequestSize(int64(sizeLimit)))
	}
}
