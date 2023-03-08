package http_in

import (
	"encoding/json"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func RegisterOperatinalRoutes(api *Api, version string, metricRegistry *prometheus.Registry) {
	metricHandler := promhttp.HandlerFor(metricRegistry, promhttp.HandlerOpts{Registry: metricRegistry})

	api.mux.Get("/version", versionHandler(version))
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

func versionHandler(version string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

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
