package app_test

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jademcosta/jiboia/pkg/app"
	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/jademcosta/jiboia/pkg/logger"
	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	confYaml, err := os.ReadFile("../../config_example.yaml")
	assert.NoError(t, err, "should not return error when reading config file")
	assert.NotEmpty(t, confYaml, "config content should not be empty")

	conf, err := config.New([]byte(confYaml))
	assert.NoError(t, err, "should not return error when creating config")

	l := logger.New(&conf.O11y.Log)
	myApp := app.New(conf, l)
	go myApp.Start()
	time.Sleep(2 * time.Second)

	response, err := http.Get("http://localhost:9099/ready")
	assert.NoError(t, err, "should return no error when calling GET on API")
	assert.Equal(t, 200, response.StatusCode, "the API return should be success")
	response.Body.Close()

	c := myApp.Stop()
	<-c
}
