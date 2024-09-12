package httpmiddleware

import (
	"log/slog"
	"net/http"
	"time"
)

type loggingMiddleware struct {
	l    *slog.Logger
	next http.Handler
}

func NewLoggingMiddleware(l *slog.Logger) func(next http.Handler) http.Handler {
	logging := &loggingMiddleware{
		l: l,
	}

	return func(next http.Handler) http.Handler {
		logging.next = next
		return logging
	}
}

func (midd *loggingMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	timeStart := time.Now()
	wrapper := &responseWriterWrapper{wrapped: w}

	midd.next.ServeHTTP(wrapper, r)

	defer func() {
		midd.l.Info("HTTP response",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapper.statusCode,
			"size", wrapper.responseSize, //TODO: append the unit on the size
			"from", r.RemoteAddr,
			"latency_time", time.Since(timeStart).String())
	}()
}
