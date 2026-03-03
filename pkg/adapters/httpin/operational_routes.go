package httpin

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterOperatinalRoutes(
	api *API,
	version string,
	metricRegistry *prometheus.Registry,
	conf config.Config,
	logg *slog.Logger,
) {
	metricHandler := promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{Registry: metricRegistry})

	api.mux.Get("/version", versionHandler(version, logg))
	api.mux.Handle("/metrics", metricHandler)

	if conf.O11y.ConfigDumpEnabled {
		api.mux.Get("/config", configHandler(conf, logg))
	}

	api.mux.Get("/healthy", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	api.mux.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//TODO: we need to check if other components are running
		//TODO: This can be used to rule the pod out of LB when it is overloaded?
	})

	if conf.O11y.Profiling.Enabled {
		api.mux.Mount("/debug", middleware.Profiler())
	}
}

func versionHandler(version string, logg *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {

		versionResponse := make(map[string]string)
		versionResponse["version"] = version

		response, err := json.Marshal(versionResponse)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logg.Error("failed to marshal version response", "error", err)
		} else {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(response) //nolint:errcheck
		}
	}
}

func configHandler(conf config.Config, logg *slog.Logger) http.HandlerFunc {
	redactedConf := conf.Redacted()

	return func(w http.ResponseWriter, _ *http.Request) {
		response, err := json.Marshal(redactedConf)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logg.Error("failed to marshal config response", "error", err)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write(response) //nolint:errcheck
		}
	}
}
