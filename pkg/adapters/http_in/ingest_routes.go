package http_in

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"

	"github.com/jademcosta/jiboia/pkg/adapters/http_in/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"go.uber.org/zap"
)

var payloadMaxSizeErr *http.MaxBytesError

func RegisterIngestingRoutes(
	api *Api,
	version string,
	flws []flow.Flow,
) {

	for _, flw := range flws {
		flwCopy := flw

		path := fmt.Sprintf("/%s/async_ingestion", flwCopy.Name)
		if flwCopy.Token != "" {
			api.mux.With(httpmiddleware.Auth(flwCopy.Token)).
				Post(path, asyncIngestion(api.log, &flwCopy))
			api.mux.With(httpmiddleware.Auth(flwCopy.Token)).
				Post("/"+version+path, asyncIngestion(api.log, &flwCopy))
		} else {
			api.mux.Post(path, asyncIngestion(api.log, &flwCopy))
			api.mux.Post("/"+version+path, asyncIngestion(api.log, &flwCopy))
		}
	}

	api.mux.Middlewares()
}

func asyncIngestion(l *zap.SugaredLogger, flw *flow.Flow) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		//TODO: implement the "with" on the logger and add the "ingestion_type": "async" here on this fn

		currentPath := r.URL.Path
		buf := &bytes.Buffer{}
		dataLen, err := buf.ReadFrom(r.Body)

		observeSize(currentPath, float64(dataLen))

		if err != nil {
			l.Warnw("async http request failed", "error", err)
			//TODO: send a JSON response with the error
			//TODO: which should be the response in this case?
			if errors.As(err, &payloadMaxSizeErr) {
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				increaseErrorCount("request_entity_too_large", currentPath)
				return
			}
			w.WriteHeader(http.StatusBadRequest)
			increaseErrorCount("error_reading_body", currentPath)
			return
		}

		if dataLen <= 0 {
			l.Warn("request without body, ignoring")
			w.Header().Set("Content-Type", "application/json")
			//TODO: send a JSON response with the error
			//TODO: which should be the response in this case?
			w.WriteHeader(http.StatusBadRequest)
			increaseErrorCount("request_without_body", currentPath)
			return
		}

		data := buf.Bytes()

		l.Debugw("data received on async handler", "length", dataLen)

		err = flw.Entrypoint.Enqueue(data)
		if err != nil {
			l.Warnw("failed while enqueueing data from http request", "error", err)
			//TODO: send a JSON response with the error
			w.WriteHeader(http.StatusInternalServerError)
			increaseErrorCount("enqueue_failed", currentPath)
			return
		}

		//TODO: Maybe send back a JSON with the length of the content read?
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
	}
}
