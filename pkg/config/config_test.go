package config_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jademcosta/jiboia/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestDefaultValues(t *testing.T) {
	configYaml := `
  flows:
    - name: "my-flow"
`

	conf, err := config.New([]byte(configYaml))
	if err != nil {
		assert.Fail(t, "should create a config %v", err)
	}

	assert.Equal(t, "json", conf.O11y.Log.Format, "default for o11y.log.format config doesn't match")
	assert.Equal(t, "info", conf.O11y.Log.Level, "default for o11y.log.level config doesn't match")
	assert.Equal(t, false, conf.O11y.TracingEnabled, "default for o11y.tracing_enabled is false")
	assert.Equal(t, 9199, conf.API.Port, "default for api.port config doesn't match")
	assert.Equal(t, 50, conf.Flows[0].MaxConcurrentUploads,
		"default for flow.max_concurrent_uploads config doesn't match")
	assert.Equal(t, 1, conf.Flows[0].PathPrefixCount,
		"default value for flow path_prefix_count should be 1")
	assert.Empty(t, conf.Flows[0].Ingestion.Decompression.ActiveDecompressions,
		"default flows.ingestion.decompress.active is empty")
	assert.Equal(t, 0, conf.Flows[0].Ingestion.Decompression.MaxConcurrency,
		"default flows.ingestion.decompress.max_concurrency is zero")
	assert.Equal(t, false, conf.Flows[0].Ingestion.CircuitBreaker.Disable,
		"default flows.ingestion.circuit_breaker.disable is false")
	assert.Equal(t, int64(100), conf.Flows[0].Ingestion.CircuitBreaker.OpenInterval,
		"default flows.ingestion.circuit_breaker.open_iterval is 100")
	assert.Equal(t, "512", conf.Flows[0].Ingestion.Decompression.InitialBufferSize,
		"default flows.ingestion.decompression.initial_buffer_size is 512")
	assert.Equal(t, 2, conf.Flows[0].Compression.PreallocSlicePercentage,
		"default flows.compression.prealloc_slice_percentage is 2")

	sizeInBytes, err := conf.API.PayloadSizeLimitInBytes()
	assert.Equal(t, int64(0), sizeInBytes, "default for api.payload_size_limit config doesn't match")
	assert.NoError(t, err, "api.payload_size_limit should yield no error")
}

func TestConfigParsing(t *testing.T) {
	configYaml := `
o11y:
  tracing_enabled: true
  log:
    level: warn
    format: json

api:
  port: 9099
  payload_size_limit: 2MB

flows:
  - name: "my-flow"
    in_memory_queue_max_size: 1000
    max_concurrent_uploads: 50
    path_prefix_count: 7
    ingestion:
      token: "some token here!"
      decompress:
        active: ["gzip", "zstd"]
        max_concurrency: 90
    accumulator:
      size: 573MB
      separator: "_a_"
      queue_capacity: 123
      circuit_breaker:
        disable: true
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
  - name: "myflow2"
    in_memory_queue_max_size: 11
    max_concurrent_uploads: 1
    path_prefix_count: 1
    ingestion:
      circuit_breaker:
        open_interval_in_ms: 1234
    compression:
      type: gzip
      level: 2
    accumulator:
      size: 20
      separator: ""
      queue_capacity: 13
      circuit_breaker:
        open_interval_in_ms: 12345
    external_queue:
      type: noop
    object_storage:
      type: localstorage
      config:
        path: "/tmp/path/storage"
  - name: "myflow3"
    in_memory_queue_max_size: 1
    max_concurrent_uploads: 1
    path_prefix_count: 1
    ingestion:
      circuit_breaker:
        open_interval_in_ms: 12
    external_queue:
      type: noop
    object_storage:
      type: localstorage
      config:
        path: "/tmp/path/storage2"
`

	conf, err := config.New([]byte(configYaml))
	if err != nil {
		assert.Fail(t, "should create a config %v", err)
	}

	assert.Equal(t, "warn", conf.O11y.Log.Level, "should have parsed the correct o11y.log.level")
	assert.Equal(t, "json", conf.O11y.Log.Format, "should have parsed the correct o11y.log.format")
	assert.Equal(t, true, conf.O11y.TracingEnabled, "should have parsed the correct o11y.tracing_enabled")

	assert.Equal(t, 9099, conf.API.Port, "should have parsed the correct api.port")

	sizeInBytes, err := conf.API.PayloadSizeLimitInBytes()
	assert.Equal(t, int64(2097152), sizeInBytes, "should have parsed the correct api.payload_size_limit")
	assert.NoError(t, err, "api.payload_size_limit should yield no error")

	assert.Equal(t, "my-flow", conf.Flows[0].Name, "should have parsed the correct flow.name")
	assert.Equal(t, 1000, conf.Flows[0].QueueMaxSize, "should have parsed the correct flow.in_memory_queue_max_size")
	assert.Equal(t, 50, conf.Flows[0].MaxConcurrentUploads, "should have parsed the correct flow.max_concurrent_uploads")
	assert.Equal(t, 7, conf.Flows[0].PathPrefixCount, "should have parsed the correct flow.path_prefix_count")
	assert.Equal(t, "", conf.Flows[0].Compression.Type, "should have an empty flow.compression.type")
	assert.Equal(t, "", conf.Flows[0].Compression.Level, "should have an empty flow.compression.level")

	assert.Equal(t, "some token here!", conf.Flows[0].Ingestion.Token, "should have parsed the correct flow.ingestion.token")
	assert.Equal(t, []string{"gzip", "zstd"}, conf.Flows[0].Ingestion.Decompression.ActiveDecompressions,
		"should have parsed the correct flow.ingestion.decompress.active")
	assert.Equal(t, 90, conf.Flows[0].Ingestion.Decompression.MaxConcurrency,
		"should have parsed the correct flow.ingestion.decompress.max_concurrency")
	assert.Equal(t, int64(100), conf.Flows[0].Ingestion.CircuitBreaker.OpenInterval,
		"should have set the default flow.ingestion.circuit_breaker.open_interval_in_ms")
	assert.Equal(t, false, conf.Flows[0].Ingestion.CircuitBreaker.Disable,
		"should have set the default flow.ingestion.circuit_breaker.disable")

	assert.Equal(t, "573MB", conf.Flows[0].Accumulator.Size,
		"should have parsed the correct flow.accumulator.size")
	sizeAsBytes, err := conf.Flows[0].Accumulator.SizeAsBytes()
	assert.NoError(t, err, "should return no error")
	assert.Equal(t, int64(600834048), sizeAsBytes, "should know how to conver the size of accumulator")
	assert.Equal(t, "_a_", conf.Flows[0].Accumulator.Separator,
		"should have parsed the correct flow.accumulator.separator")
	assert.Equal(t, 123, conf.Flows[0].Accumulator.QueueCapacity,
		"should have parsed the correct flow.accumulator.queue_capacity")
	assert.Equal(t, true, conf.Flows[0].Accumulator.CircuitBreaker.Disable,
		"should have parsed the correct flow.accumulator.circuit_breaker.disable")
	assert.Equal(t, int64(100), conf.Flows[0].Accumulator.CircuitBreaker.OpenInterval,
		"should have set the default flow.accumulator.circuit_breaker.open_interval_in_ms")

	assert.Equal(t, "sqs", conf.Flows[0].ExternalQueue.Type, "should have parsed the correct flow.external_queue.type")
	assert.NotNil(t, conf.Flows[0].ExternalQueue.Config, "should maintain the value of flow.external_queue.config")

	assert.Equal(t, "s3", conf.Flows[0].ObjectStorage.Type, "should have parsed the correct flow.object_storage.type")
	assert.NotNil(t, conf.Flows[0].ObjectStorage.Config, "should maintain the value of flow.object_storage.config")

	assert.Equal(t, "myflow2", conf.Flows[1].Name, "should have parsed the correct flow.name")
	assert.Equal(t, 11, conf.Flows[1].QueueMaxSize, "should have parsed the correct flow.in_memory_queue_max_size")
	assert.Equal(t, 1, conf.Flows[1].MaxConcurrentUploads, "should have parsed the correct flow.max_concurrent_uploads")
	assert.Equal(t, 1, conf.Flows[1].PathPrefixCount, "should have parsed the correct flow.path_prefix_count")
	assert.Equal(t, "gzip", conf.Flows[1].Compression.Type, "should have parsed the correct flow.compression.type")
	assert.Equal(t, "2", conf.Flows[1].Compression.Level, "should have parsed the correct flow.compression.level")

	assert.Equal(t, "", conf.Flows[1].Ingestion.Token, "should have parsed the correct flow.ingestion.token (which is empty)")
	assert.Equal(t, []string(nil), conf.Flows[1].Ingestion.Decompression.ActiveDecompressions,
		"should have parsed the correct flow.ingestion.decompress.active (which is empty)")
	assert.Equal(t, 0, conf.Flows[1].Ingestion.Decompression.MaxConcurrency,
		"should have parsed the correct flow.ingestion.decompress.max_concurrency")
	assert.Equal(t, false, conf.Flows[1].Ingestion.CircuitBreaker.Disable,
		"should have parsed the correct flow.accumulator.circuit_breaker.disable")
	assert.Equal(t, int64(1234), conf.Flows[1].Ingestion.CircuitBreaker.OpenInterval,
		"should have set the default flow.accumulator.circuit_breaker.open_interval_in_ms")

	assert.Equal(t, "20", conf.Flows[1].Accumulator.Size, "should have parsed the correct flow.accumulator.size")
	sizeAsBytes, err = conf.Flows[1].Accumulator.SizeAsBytes()
	assert.NoError(t, err, "should not error")
	assert.Equal(t, int64(20), sizeAsBytes, "should know how to conver the size of accumulator")
	assert.Equal(t, "", conf.Flows[1].Accumulator.Separator, "should have parsed the correct flow.accumulator.separator")
	assert.Equal(t, 13, conf.Flows[1].Accumulator.QueueCapacity, "should have parsed the correct flow.accumulator.queue_capacity")
	assert.Equal(t, int64(12345), conf.Flows[1].Accumulator.CircuitBreaker.OpenInterval,
		"should have parsed the correct flow.accumulator.circuit_breaker.open_interval_in_ms")
	assert.Equal(t, false, conf.Flows[1].Accumulator.CircuitBreaker.Disable,
		"should have parsed the correct flow.accumulator.circuit_breaker.disable")

	assert.Equal(t, "noop", conf.Flows[1].ExternalQueue.Type, "should have parsed the correct flow.external_queue.type")
	assert.Nil(t, conf.Flows[1].ExternalQueue.Config, "should maintain the value of flow.external_queue.config")

	assert.Equal(t, "localstorage", conf.Flows[1].ObjectStorage.Type, "should have parsed the correct flow.object_storage.type")
	assert.NotNil(t, conf.Flows[1].ObjectStorage.Config, "should maintain the value of flow.object_storage.config")

	assert.NotNil(t, conf.Flows[2].Accumulator.QueueCapacity,
		"should have zero value on flow.accumulator.queue_capacity when an accumulator is not declared in a flow")
}

func TestValidateLogLevelValues(t *testing.T) {
	logLevelTemplate := `
o11y:
  log:
    level: %s
flows:
  - name: first_f`

	type testCase struct {
		conf        string
		shouldError bool
	}

	testCases := []testCase{
		{
			shouldError: true,
			conf:        fmt.Sprintf(logLevelTemplate, "nooooo"),
		},
		{
			shouldError: false,
			conf:        fmt.Sprintf(logLevelTemplate, "debug"),
		},
		{
			shouldError: false,
			conf:        fmt.Sprintf(logLevelTemplate, "info"),
		},
		{
			shouldError: false,
			conf:        fmt.Sprintf(logLevelTemplate, "warn"),
		},
		{
			shouldError: false,
			conf:        fmt.Sprintf(logLevelTemplate, "error"),
		},
	}

	for _, tc := range testCases {
		_, err := config.New([]byte(tc.conf))
		if tc.shouldError {
			assert.Errorf(t, err, "errors when log level is %s", tc.conf)
		} else {
			assert.NoErrorf(t, err, "doesn't error when log level is %s", tc.conf)
		}
	}
}

func TestErrorOnEmptyFlows(t *testing.T) {
	configYaml := `
log:
  level: warn
`

	_, err := config.New([]byte(configYaml))
	assert.Errorf(t, err, "return error when no flows is declared")
	assert.Equal(t, "at least one flow should be declared", err.Error(), "error string should make the error explicit")

	configEmptyFlowYaml := `
log:
  level: warn

flows:
`
	_, err = config.New([]byte(configEmptyFlowYaml))
	assert.Errorf(t, err, "return error when flows are empty")
	assert.Equal(t, "at least one flow should be declared", err.Error(), "error string should make the error explicit")
}

func TestErrorOnEmptyFlowName(t *testing.T) {
	configFlowWithNoNameYaml := `
log:
  level: warn

flows:
  - type: async
    in_memory_queue_max_size: 1000
    max_concurrent_uploads: 50
    max_retries: 3
    path_prefix_count: 7
    accumulator:
      size_in_bytes: 2097152 # 2MB
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
  - name: flow_2
    type: async
    in_memory_queue_max_size: 1000
    max_concurrent_uploads: 50
    max_retries: 3
    path_prefix_count: 7
    accumulator:
      size_in_bytes: 2097152 # 2MB
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
	_, err := config.New([]byte(configFlowWithNoNameYaml))
	assert.Errorf(t, err, "return error when at least one flow has no name")
	assert.Equal(t, "all flows must have a name", err.Error(), "error string should make the error explicit")
}

func TestErrorOnSpaceInFlowName(t *testing.T) {
	configFlowWithNoNameYaml := `
log:
  level: warn

flows:
  - name: my flow1
  - name: myflow2
  - name: my_flow_3
`
	_, err := config.New([]byte(configFlowWithNoNameYaml))
	assert.Errorf(t, err, "return error when flows have repeated names")
	assert.Equal(t, "flow name must not have spaces", err.Error(),
		"error string should make the error explicit")
}

func TestErrorOnRepeatedFlowName(t *testing.T) {
	configFlowWithNoNameYaml := `
log:
  level: warn

flows:
  - name: myflow1
  - name: my_flow_2
  - name: my_flow_3
  - name: myflow1
  - name: myflow123
`
	_, err := config.New([]byte(configFlowWithNoNameYaml))
	assert.Errorf(t, err, "return error when flows have repeated names")
	assert.Equal(t, "flow names must be unique", err.Error(), "error string should make the error explicit")
}

func TestApiPayloadSizeLimitParsingErrors(t *testing.T) {

	configYamlTemplate := `
  api:
    payload_size_limit: {{PAYLOAD_SIZE}}
  flows:
    - name: "my-flow"
`

	testCases := []struct {
		size string
	}{
		{size: "a"},
		{size: "12io"},
		{size: "kb"},
		{size: "KB"},
		{size: "mb"},
		{size: "amb"},
		{size: "AGB"},
		{size: "kb12kb"},
		{size: "a12kb"},
	}

	for _, tc := range testCases {
		configYaml := strings.Replace(configYamlTemplate, "{{PAYLOAD_SIZE}}", tc.size, 1)
		_, err := config.New([]byte(configYaml))
		assert.Errorf(t, err, "should return error on config creation when api size limit is %s", tc.size)
	}
}

func TestApiPayloadSizeLimitValidation(t *testing.T) {
	testCases := []struct {
		size        string
		shouldError bool
	}{
		{size: "a", shouldError: true},
		{size: "12io", shouldError: true},
		{size: "kb", shouldError: true},
		{size: "KB", shouldError: true},
		{size: "mb", shouldError: true},
		{size: "amb", shouldError: true},
		{size: "AGB", shouldError: true},
		{size: "kb12kb", shouldError: true},
		{size: "a12kb", shouldError: true},

		{size: "0", shouldError: false},
		{size: "", shouldError: false},
		{size: "1", shouldError: false},
		{size: "2", shouldError: false},
		{size: "22", shouldError: false},
		{size: "278465", shouldError: false},
		{size: "1111111111", shouldError: false},
		{size: "187349873947", shouldError: false},
		{size: "1kb", shouldError: false},
		{size: "2KB", shouldError: false},
		{size: "1024KB", shouldError: false},
		{size: "5000kb", shouldError: false},
		{size: "5000mb", shouldError: false},
		{size: "373gb", shouldError: false},
		{size: "373pb", shouldError: false},
		{size: "373eb", shouldError: false},
	}

	for _, tc := range testCases {
		conf := config.APIConfig{PayloadSizeLimit: tc.size}

		_, err := conf.PayloadSizeLimitInBytes()
		if tc.shouldError {
			assert.Errorf(t, err, "size %s should result in error", tc.size)
		} else {
			assert.NoErrorf(t, err, "size %s should NOT result in error", tc.size)
		}
	}
}

func TestErrorOnInvalidLogFormat(t *testing.T) {
	//TODO: implement me
}
