package httpmiddleware

import (
	"fmt"
	"net/http"
)

func Auth(token string) func(http.Handler) http.Handler {

	formatedToken := fmt.Sprintf("Bearer %s", token)

	f := func(h http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			reqToken := r.Header.Get("Authorization")

			if reqToken != formatedToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			h.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
	return f
}
