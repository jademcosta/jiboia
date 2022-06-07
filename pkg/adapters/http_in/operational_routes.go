package http_in

import (
	"encoding/json"
	"net/http"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterOperatinalRoutes(api *Api, c *config.Config, metricRegistry *prometheus.Registry) {
	metricHandler := promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{Registry: metricRegistry})

	api.mux.Get("/version", versionHandler(c))
	api.mux.Handle("/metrics", metricHandler)

	api.mux.Get("/healthy", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	api.mux.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		//TODO: we need to check if other components are running
		//TODO: This can be used to rule the pod out of LB when it is overloaded?
	})
}

func versionHandler(c *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		versionResponse := make(map[string]string)
		versionResponse["version"] = c.Version

		response, err := json.Marshal(versionResponse)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			//TODO: log the error
		} else {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			w.Write(response)
		}
	}
}
