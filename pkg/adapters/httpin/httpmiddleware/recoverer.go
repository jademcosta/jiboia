package httpmiddleware

import (
	"log/slog"
	"net/http"
)

type recovererMiddleware struct {
	l    *slog.Logger
	next http.Handler
}

func NewRecoverer(l *slog.Logger) func(next http.Handler) http.Handler {
	recoverer := &recovererMiddleware{
		l: l,
	}

	return func(next http.Handler) http.Handler {
		recoverer.next = next
		return recoverer
	}
}

func (midd *recovererMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rvr := recover(); rvr != nil && rvr != http.ErrAbortHandler {

			midd.l.Error("captured panic on HTTP request", "error", rvr)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	midd.next.ServeHTTP(w, r)
}
