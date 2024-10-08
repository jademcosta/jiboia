package httpin

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/jademcosta/jiboia/pkg/adapters/httpin/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/circuitbreaker"
	"github.com/jademcosta/jiboia/pkg/compression"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/domain/flow"
)

var payloadMaxSizeErr *http.MaxBytesError

type ingestionRoute struct {
	l                              *slog.Logger
	flw                            *flow.Flow
	decompressionSemaphor          chan struct{}
	decompressionInitialBufferSize int
	validDecompressionAlgorithms   map[string]struct{}
	circuitBreaker                 circuitbreaker.TwoStepCircuitBreaker
}

func newIngestionRoute(l *slog.Logger, flw *flow.Flow) *ingestionRoute {

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
		l:                              l,
		flw:                            flw,
		validDecompressionAlgorithms:   validDecompressionAlgorithms,
		decompressionSemaphor:          decompressionSemaphor,
		decompressionInitialBufferSize: flw.DecompressionInitialBufferSizeBytes,
		circuitBreaker:                 flw.CircuitBreaker,
	}
}

func (handler *ingestionRoute) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	currentPath := r.URL.Path

	enqueuedWithSuccess, err := handler.circuitBreaker.Allow()
	circuitOpen := err != nil
	if circuitOpen {
		w.WriteHeader(http.StatusInternalServerError)
		increaseErrorCount("circuit_breaker_open", currentPath)
		return
	}

	buf := &bytes.Buffer{}
	dataLen, err := buf.ReadFrom(r.Body)
	observeSize(currentPath, float64(dataLen))

	if err != nil && errors.As(err, &payloadMaxSizeErr) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		increaseErrorCount("request_entity_too_large", currentPath)
		return
	} else if err != nil {
		handler.l.Warn("error reading body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		increaseErrorCount("error_reading_body", currentPath)
		return
	}

	if dataLen <= 0 {
		handler.l.Warn("request without body, ignoring")
		w.WriteHeader(http.StatusBadRequest)
		increaseErrorCount("request_without_body", currentPath)
		return
	}

	data := buf.Bytes()
	handler.l.Debug("data received on async handler", "length", dataLen)

	decompressAlgorithm :=
		selectDecompressionAlgorithm(handler.validDecompressionAlgorithms, r.Header["Content-Encoding"])

	if decompressAlgorithm != "" {
		token, needToReturnToken := <-handler.decompressionSemaphor
		data, err = decompress(data, decompressAlgorithm, r.URL.Path, handler.decompressionInitialBufferSize)
		if needToReturnToken {
			handler.decompressionSemaphor <- token
		}
		if err != nil {
			handler.l.Warn("failed to decompress data", "algorithm", decompressAlgorithm, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			increaseErrorCount("enqueue_failed", currentPath)
			return
		}
	}

	err = handler.flw.Entrypoint.Enqueue(data)
	if err != nil {
		handler.l.Warn("failed while enqueueing data from http request", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		increaseErrorCount("enqueue_failed", currentPath)
		enqueuedWithSuccess(false)
		return
	}

	enqueuedWithSuccess(true)
	w.WriteHeader(http.StatusOK)
}

func RegisterIngestingRoutes(
	api *API,
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

func asyncIngestion(l *slog.Logger, flw *flow.Flow) http.HandlerFunc {

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

func decompress(data []byte, algorithm string, pathForMetrics string, bufferSize int) ([]byte, error) {
	increaseDecompressionCount(algorithm)
	timeStart := time.Now()

	decompressor, err := compression.NewReader(&config.CompressionConfig{Type: algorithm}, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	decompressedData, err := localReadAll(decompressor, bufferSize)
	if err != nil {
		return nil, fmt.Errorf("error decompressing data: %w", err)
	}

	err = decompressor.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing decompressor data: %w", err)
	}

	observeDecompressionTime(pathForMetrics, time.Since(timeStart))

	return decompressedData, nil
}

func localReadAll(r compression.CompressorReader, initialSliceSize int) ([]byte, error) {
	//Code copied from io.ReadAll
	b := make([]byte, 0, initialSliceSize)
	for {
		n, err := r.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return b, err
		}

		if len(b) == cap(b) {
			// Add more capacity (let append pick how much).
			b = append(b, 0)[:len(b)]
		}
	}
}
