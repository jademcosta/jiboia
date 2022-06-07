package http_in

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jademcosta/jiboia/pkg/adapters/http_in/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

type Api struct {
	mux  *chi.Mux
	log  *zap.SugaredLogger
	srv  *http.Server
	port int
}

func New(l *zap.SugaredLogger, c *config.Config, metricRegistry *prometheus.Registry, flow domain.DataFlow) *Api {

	router := chi.NewRouter()

	api := &Api{
		mux:  router,
		log:  l,
		srv:  &http.Server{Addr: fmt.Sprintf(":%d", c.Api.Port), Handler: router},
		port: c.Api.Port,
	}

	registerDefaultMiddlewares(api, l, metricRegistry)

	//TODO: I'm using static approach to be able to release it asap. In the future the route naming
	//creation needs to be dynamic
	RegisterIngestingRoutes(api, c, flow) //TODO: add middleware that will return syntax error in case a request comes with no body
	RegisterOperatinalRoutes(api, c, metricRegistry)
	api.mux.Mount("/debug", middleware.Profiler())

	return api
}

func (api *Api) ListenAndServe() error {
	api.log.Info(fmt.Sprintf("Starting HTTP server on port %d", api.port))
	return fmt.Errorf("on serving HTTP: %w", api.srv.ListenAndServe())
}

func (api *Api) Shutdown() error {
	ctx := context.Background()
	return api.srv.Shutdown(ctx)
}

func registerDefaultMiddlewares(api *Api, l *zap.SugaredLogger, metricRegistry *prometheus.Registry) {
	//Middlewares on the top wrap the ones in the bottom
	api.mux.Use(httpmiddleware.NewLoggingMiddleware("jiboia", l))
	api.mux.Use(httpmiddleware.NewMetricsMiddleware("jiboia", metricRegistry))
	api.mux.Use(httpmiddleware.NewRecoverer(l))
}
