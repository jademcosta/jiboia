package http_in

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

func RegisterIngestingRoutes(
	api *Api,
	sizeHistogram *prometheus.HistogramVec,
	flws []flow.Flow,
) {

	for _, flw := range flws {
		flwCopy := flw
		api.mux.Post(fmt.Sprintf("/%s/async_ingestion", flw.Name), asyncIngestion(api.log, sizeHistogram, &flwCopy))
	}

}

func asyncIngestion(l *zap.SugaredLogger, sizeHistogram *prometheus.HistogramVec, flw *flow.Flow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//TODO: implement the "with" on the logger and add the "ingestion_type": "async" here on this fn

		//FIXME: buffer size needs a limits when reading, to avoid OOM
		buf := &bytes.Buffer{}
		dataLen, err := buf.ReadFrom(r.Body)

		sizeHistogram.WithLabelValues(r.URL.Path).Observe(float64(dataLen))

		if err != nil {
			l.Warn("async http request failed", "error", err)
			w.Header().Set("Content-Type", "application/json")
			//TODO: send a JSON response with the error
			//TODO: which should be the response in this case?
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if dataLen <= 0 {
			l.Warn("request without body, ignoring")
			w.Header().Set("Content-Type", "application/json")
			//TODO: send a JSON response with the error
			//TODO: which should be the response in this case?
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		data := buf.Bytes()

		l.Debug("data received on async handler", "length", dataLen)

		err = flw.Entrypoint.Enqueue(data)
		if err != nil {
			l.Warn("failed while enqueueing data from http request", "error", err)
			w.Header().Set("Content-Type", "application/json")
			//TODO: send a JSON response with the error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		//TODO: Maybe send back a JSON with the length of the content read?
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}
