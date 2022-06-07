package httpmiddleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jademcosta/jiboia/pkg/adapters/http_in/httpmiddleware"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func alwaysPanicHandler(w http.ResponseWriter, r *http.Request) {
	panic("WOW, panic handler!")
}

func TestItCapturesPanicAndReturn500(t *testing.T) {

	l := logger.New(&config.Config{Log: config.LogConfig{Level: "panic", Format: "json"}})

	recoverer := httpmiddleware.NewRecoverer(l)
	recHandler := recoverer(http.HandlerFunc(alwaysPanicHandler))

	req := httptest.NewRequest(http.MethodGet, "/whatever", nil)
	w := httptest.NewRecorder()

	recHandler.ServeHTTP(w, req)

	response := w.Result()
	defer response.Body.Close()

	assert.Equalf(t, http.StatusInternalServerError, response.StatusCode, "Recover middleware should have caught panic and returned internal server error")
}

//TODO: once log interface has been created, add tests to ensure we log the error
