package httpmiddleware

import (
	"net/http"

	"go.uber.org/zap"
)

type recovererMiddleware struct {
	l    *zap.SugaredLogger
	next http.Handler
}

func NewRecoverer(l *zap.SugaredLogger) func(next http.Handler) http.Handler {
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

			midd.l.Errorw("captured panic on HTTP request", "error", rvr)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	midd.next.ServeHTTP(w, r)
}
