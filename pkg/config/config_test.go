package config_test

import (
	"fmt"
	"testing"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestDefaultValues(t *testing.T) {
	configYaml := `
flows:
  - name: floooooow
`

	conf, err := config.New([]byte(configYaml))
	if err != nil {
		assert.Fail(t, "should create a config %v", err)
	}

	assert.Equal(t, "json", conf.Log.Format, "default for log.format config doesn't match")
	assert.Equal(t, "info", conf.Log.Level, "default for log.level config doesn't match")
	assert.Equal(t, 9010, conf.Api.Port, "default for api.port config doesn't match")
	assert.Equal(t, 500, conf.Flows[0].MaxConcurrentUploads, "default for flow.max_concurrent_uploads config doesn't match")
	assert.Equal(t, 0, conf.Flows[0].MaxRetries, "default for flow.max_retries config doesn't match")
	assert.Equal(t, 1, conf.Flows[0].PathPrefixCount, "default value for flow path_prefix_count should be 1")
}

func TestConfigParsing(t *testing.T) {
	configYaml := `
log:
  level: warn
  format: yaml

api:
  port: 9099

flows:
  - name: "my-flow"
    type: async
    in_memory_queue_max_size: 1000
    max_concurrent_uploads: 50
    max_retries: 3
    path_prefix_count: 7
    accumulator:
      size_in_bytes: 2097152 # 2MB
      separator: "_a_"
      queue_capacity: 123
    external_queue:
      type: sqs
      config:
        url: some-url-here
        region: aws-region-here
        access_key: "access 1"
        secret_key: "secret 1"
    object_storage:
      type: s3
      config:
        bucket: some-bucket-name-here
        region: some-region
        endpoint: my-endpoint2
        access_key: "access 2"
        secret_key: "secret 2"
`

	conf, err := config.New([]byte(configYaml))
	if err != nil {
		assert.Fail(t, "should create a config %v", err)
	}

	assert.Equal(t, "warn", conf.Log.Level, "should have parsed the correct log.level")
	assert.Equal(t, "yaml", conf.Log.Format, "should have parsed the correct log.format") // TODO: allow other formats

	assert.Equal(t, 9099, conf.Api.Port, "should have parsed the correct api.port")

	assert.Equal(t, "my-flow", conf.Flows[0].Name, "should have parsed the correct flow.name")
	assert.Equal(t, "async", conf.Flows[0].Type, "should have parsed the correct flow.type")
	assert.Equal(t, 1000, conf.Flows[0].QueueMaxSize, "should have parsed the correct flow.in_memory_queue_max_size")
	assert.Equal(t, 50, conf.Flows[0].MaxConcurrentUploads, "should have parsed the correct flow.max_concurrent_uploads")
	assert.Equal(t, 3, conf.Flows[0].MaxRetries, "should have parsed the correct flow.max_retries")
	assert.Equal(t, 7, conf.Flows[0].PathPrefixCount, "should have parsed the correct flow.path_prefix_count")

	assert.Equal(t, 2097152, conf.Flows[0].Accumulator.SizeInBytes, "should have parsed the correct flow.accumulator.size_in_bytes")
	assert.Equal(t, "_a_", conf.Flows[0].Accumulator.Separator, "should have parsed the correct flow.accumulator.separator")
	assert.Equal(t, 123, conf.Flows[0].Accumulator.QueueCapacity, "should have parsed the correct flow.accumulator.queue_capacity")

	assert.Equal(t, "sqs", conf.Flows[0].ExternalQueue.Type, "should have parsed the correct flow.external_queue.type")
	assert.NotNil(t, conf.Flows[0].ExternalQueue.Config, "should maintain the value of flow.external_queue.config")

	assert.Equal(t, "s3", conf.Flows[0].ObjectStorage.Type, "should have parsed the correct flow.object_storage.type")
	assert.NotNil(t, conf.Flows[0].ObjectStorage.Config, "should maintain the value of flow.object_storage.config")
}

func TestValidateLogLevelValues(t *testing.T) {
	logLevelTemplate := `
log:
  level: %s
flows:
  - name: flowwwssss`

	type testCase struct {
		conf        string
		shouldPanic bool
	}

	testCases := []testCase{
		{
			shouldPanic: true,
			conf:        fmt.Sprintf(logLevelTemplate, "nooooo"),
		},
		{
			shouldPanic: false,
			conf:        fmt.Sprintf(logLevelTemplate, "debug"),
		},
		{
			shouldPanic: false,
			conf:        fmt.Sprintf(logLevelTemplate, "info"),
		},
		{
			shouldPanic: false,
			conf:        fmt.Sprintf(logLevelTemplate, "warn"),
		},
		{
			shouldPanic: false,
			conf:        fmt.Sprintf(logLevelTemplate, "error"),
		},
	}

	for _, tc := range testCases {
		if tc.shouldPanic {
			assert.Panics(t, func() { config.New([]byte(tc.conf)) }, "should panic on not allowed values.")
		} else {
			_, err := config.New([]byte(tc.conf))
			if err != nil {
				assert.Fail(t, "should have success creating a config %v", err)
			}
		}
	}
}

func TestPanicIfNoFlow(t *testing.T) {
	configYaml := `
log:
  level: warn
  format: yaml

api:
  port: 9099
`

	assert.Panics(t, func() { config.New([]byte(configYaml)) }, "should panic when no flow is declared.")
}
