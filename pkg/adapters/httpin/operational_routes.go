package httpin

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterOperatinalRoutes(api *API, version string, metricRegistry *prometheus.Registry) {
	metricHandler := promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{Registry: metricRegistry})

	api.mux.Get("/version", versionHandler(version))
	api.mux.Handle("/metrics", metricHandler)

	api.mux.Get("/healthy", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	api.mux.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		//TODO: we need to check if other components are running
		//TODO: This can be used to rule the pod out of LB when it is overloaded?
	})

	api.mux.Mount("/debug", middleware.Profiler())
}

func versionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {

		versionResponse := make(map[string]string)
		versionResponse["version"] = version

		response, err := json.Marshal(versionResponse)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			//TODO: log the error
		} else {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(response) //nolint:errcheck
		}
	}
}
