package httpmiddleware

import "net/http"

type responseWriterWrapper struct {
	wrapped      http.ResponseWriter
	statusCode   int
	responseSize int
}

func (w *responseWriterWrapper) Header() http.Header {
	return w.wrapped.Header()
}

func (w *responseWriterWrapper) Write(data []byte) (int, error) {
	w.responseSize = len(data)
	return w.wrapped.Write(data)
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.wrapped.WriteHeader(statusCode)
}
