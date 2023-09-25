package http_in

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/jademcosta/jiboia/pkg/adapters/http_in/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/compressor"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
	"go.uber.org/zap"
)

var payloadMaxSizeErr *http.MaxBytesError

type ingestionRoute struct {
	l                            *zap.SugaredLogger
	flw                          *flow.Flow
	decompressionSemaphor        chan struct{}
	validDecompressionAlgorithms map[string]struct{}
}

func newIngestionRoute(l *zap.SugaredLogger, flw *flow.Flow) *ingestionRoute {

	validDecompressionAlgorithms := make(map[string]struct{})
	for _, alg := range flw.DecompressionAlgorithms {
		validDecompressionAlgorithms[alg] = struct{}{}
	}

	var decompressionSemaphor chan struct{}
	if flw.DecompressionMaxConcurrency != 0 {
		decompressionSemaphor = make(chan struct{}, flw.DecompressionMaxConcurrency)
		for i := 0; i < flw.DecompressionMaxConcurrency; i++ {
			decompressionSemaphor <- struct{}{}
		}
	} else {
		decompressionSemaphor = make(chan struct{})
		close(decompressionSemaphor)
	}

	return &ingestionRoute{
		l:                            l,
		flw:                          flw,
		validDecompressionAlgorithms: validDecompressionAlgorithms,
		decompressionSemaphor:        decompressionSemaphor,
	}
}

func (handler *ingestionRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	currentPath := r.URL.Path
	buf := &bytes.Buffer{}
	dataLen, err := buf.ReadFrom(r.Body)

	observeSize(currentPath, float64(dataLen))

	if err != nil {
		handler.l.Warnw("async http request failed", "error", err)
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
		handler.l.Warn("request without body, ignoring")
		w.Header().Set("Content-Type", "application/json")
		//TODO: which should be the response in this case?
		w.WriteHeader(http.StatusBadRequest)
		increaseErrorCount("request_without_body", currentPath)
		return
	}

	data := buf.Bytes()
	handler.l.Debugw("data received on async handler", "length", dataLen)

	decompressAlgorithm :=
		selectDecompressionAlgorithm(handler.validDecompressionAlgorithms, r.Header["Content-Encoding"])

	if decompressAlgorithm != "" {

		token, needToReturnToken := <-handler.decompressionSemaphor
		data, err = decompress(data, decompressAlgorithm)
		if needToReturnToken {
			handler.decompressionSemaphor <- token
		}
		if err != nil {
			handler.l.Warnw("failed to decompress data", "algorithm", decompressAlgorithm, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			increaseErrorCount("enqueue_failed", currentPath)
			return
		}
	}

	err = handler.flw.Entrypoint.Enqueue(data)
	if err != nil {
		handler.l.Warnw("failed while enqueueing data from http request", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		increaseErrorCount("enqueue_failed", currentPath)
		return
	}

	//TODO: Maybe send back a JSON with the length of the content read?
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func RegisterIngestingRoutes(
	api *Api,
	version string,
	flws []flow.Flow,
) {

	for _, flw := range flws {
		flwCopy := flw
		path := fmt.Sprintf("/%s/async_ingestion", flwCopy.Name)
		middlewares := make([]func(http.Handler) http.Handler, 0, 1)

		if flwCopy.Token != "" {
			middlewares = append(middlewares, httpmiddleware.Auth(flwCopy.Token))
		}

		api.mux.With(middlewares...).
			Post(path, asyncIngestion(api.log, &flwCopy))
		api.mux.With(middlewares...).
			Post("/"+version+path, asyncIngestion(api.log, &flwCopy))
	}

	api.mux.Middlewares()
}

func asyncIngestion(l *zap.SugaredLogger, flw *flow.Flow) http.HandlerFunc {

	h := newIngestionRoute(l, flw)

	return func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
	}
}

func selectDecompressionAlgorithm(acceptedAlgorithms map[string]struct{}, contentEncodingHeaders []string) string {
	if len(acceptedAlgorithms) <= 0 || len(contentEncodingHeaders) <= 0 {
		return ""
	}

	for _, contentType := range contentEncodingHeaders {
		if _, exists := acceptedAlgorithms[contentType]; exists {
			return contentType
		}
	}

	return ""
}

func decompress(data []byte, algorithm string) ([]byte, error) {

	decompressor, err := compressor.NewReader(&config.Compression{Type: algorithm}, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	decompressedData, err := io.ReadAll(decompressor)
	if err != nil {
		return nil, err
	}

	return decompressedData, nil
}
