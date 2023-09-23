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

	assert.Equal(t, "json", conf.Log.Format, "default for log.format config doesn't match")
	assert.Equal(t, "info", conf.Log.Level, "default for log.level config doesn't match")
	assert.Equal(t, 9010, conf.Api.Port, "default for api.port config doesn't match")
	assert.Equal(t, 500, conf.Flows[0].MaxConcurrentUploads, "default for flow.max_concurrent_uploads config doesn't match")
	assert.Equal(t, 1, conf.Flows[0].PathPrefixCount, "default value for flow path_prefix_count should be 1")
	assert.Empty(t, conf.Flows[0].Ingestion.Decompression.ActiveDecompressions, "default flows.ingestion.decompress_ingested_data is empty")

	sizeInBytes, err := conf.Api.PayloadSizeLimitInBytes()
	assert.Equal(t, 0, sizeInBytes, "default for api.payload_size_limit config doesn't match")
	assert.NoError(t, err, "api.payload_size_limit should yield no error")
}

func TestConfigParsing(t *testing.T) {
	configYaml := `
log:
  level: warn
  format: yaml

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
      size_in_bytes: 2097152
      separator: "_a_"
      queue_capacity: 123
      circuit_breaker:
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
    compression:
      type: gzip
      level: 2
    accumulator:
      size_in_bytes: 20
      separator: ""
      queue_capacity: 13
      circuit_breaker:
        turn_on: true
        open_interval_in_ms: 12345
    external_queue:
      type: noop
    object_storage:
      type: localstorage
      config:
        path: "/tmp/path/storage"
`

	conf, err := config.New([]byte(configYaml))
	if err != nil {
		assert.Fail(t, "should create a config %v", err)
	}

	assert.Equal(t, "warn", conf.Log.Level, "should have parsed the correct log.level")
	assert.Equal(t, "yaml", conf.Log.Format, "should have parsed the correct log.format") // TODO: allow other formats

	assert.Equal(t, 9099, conf.Api.Port, "should have parsed the correct api.port")

	sizeInBytes, err := conf.Api.PayloadSizeLimitInBytes()
	assert.Equal(t, 2097152, sizeInBytes, "should have parsed the correct api.payload_size_limit")
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

	assert.Equal(t, 2097152, conf.Flows[0].Accumulator.SizeInBytes, "should have parsed the correct flow.accumulator.size_in_bytes")
	assert.Equal(t, "_a_", conf.Flows[0].Accumulator.Separator, "should have parsed the correct flow.accumulator.separator")
	assert.Equal(t, 123, conf.Flows[0].Accumulator.QueueCapacity, "should have parsed the correct flow.accumulator.queue_capacity")
	assert.Equal(t, map[string]string(nil), conf.Flows[0].Accumulator.CircuitBreaker, "should have parsed the correct flow.accumulator.circuit_breaker")

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

	assert.Equal(t, 20, conf.Flows[1].Accumulator.SizeInBytes, "should have parsed the correct flow.accumulator.size_in_bytes")
	assert.Equal(t, "", conf.Flows[1].Accumulator.Separator, "should have parsed the correct flow.accumulator.separator")
	assert.Equal(t, 13, conf.Flows[1].Accumulator.QueueCapacity, "should have parsed the correct flow.accumulator.queue_capacity")
	assert.Equal(t, map[string]string{"turn_on": "true", "open_interval_in_ms": "12345"}, conf.Flows[1].Accumulator.CircuitBreaker, "should have parsed the correct flow.accumulator.circuit_breaker")

	assert.Equal(t, "noop", conf.Flows[1].ExternalQueue.Type, "should have parsed the correct flow.external_queue.type")
	assert.Nil(t, conf.Flows[1].ExternalQueue.Config, "should maintain the value of flow.external_queue.config")

	assert.Equal(t, "localstorage", conf.Flows[1].ObjectStorage.Type, "should have parsed the correct flow.object_storage.type")
	assert.NotNil(t, conf.Flows[1].ObjectStorage.Config, "should maintain the value of flow.object_storage.config")
}

func TestValidateLogLevelValues(t *testing.T) {
	logLevelTemplate := `
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

func TestApiPayloadSizeLimitParsing(t *testing.T) {
	configYamlTemplate := `
  api:
    payload_size_limit: {{PAYLOAD_SIZE}}
  flows:
    - name: "my-flow"
`

	testCases := []struct {
		size        string
		sizeInBytes int
	}{
		{size: "0", sizeInBytes: 0},
		{size: "", sizeInBytes: 0},
		{size: "1", sizeInBytes: 1},
		{size: "9", sizeInBytes: 9},
		{size: "99", sizeInBytes: 99},
		{size: "278465", sizeInBytes: 278465},
		{size: "1111111111", sizeInBytes: 1111111111},
		{size: "187349873947", sizeInBytes: 187349873947},

		{size: "1kb", sizeInBytes: 1024},
		{size: "1KB", sizeInBytes: 1024},
		{size: "2KB", sizeInBytes: 2048},
		{size: "1024KB", sizeInBytes: 1048576},
		{size: "5000kb", sizeInBytes: 5120000},

		{size: "1mb", sizeInBytes: 1048576},
		{size: "2mb", sizeInBytes: 2 * 1048576},
		{size: "2MB", sizeInBytes: 2 * 1048576},
		{size: "110MB", sizeInBytes: 110 * 1048576},

		{size: "1gb", sizeInBytes: 1073741824},
		{size: "1GB", sizeInBytes: 1073741824},
		{size: "2gb", sizeInBytes: 2 * 1073741824},
		{size: "20gb", sizeInBytes: 20 * 1073741824},

		{size: "1pb", sizeInBytes: 1099511627776},
		{size: "1PB", sizeInBytes: 1099511627776},
		{size: "2PB", sizeInBytes: 2 * 1099511627776},
		{size: "23PB", sizeInBytes: 23 * 1099511627776},
		{size: "530PB", sizeInBytes: 530 * 1099511627776},

		{size: "1eb", sizeInBytes: 1125899906842624},
		{size: "1EB", sizeInBytes: 1125899906842624},
		{size: "2EB", sizeInBytes: 2 * 1125899906842624},
		{size: "15EB", sizeInBytes: 15 * 1125899906842624},
	}

	for _, tc := range testCases {
		configYaml := strings.Replace(configYamlTemplate, "{{PAYLOAD_SIZE}}", tc.size, 1)
		conf, err := config.New([]byte(configYaml))
		if err != nil {
			assert.Fail(t, "should create a config %v", err)
		}

		sizeInBytes, err := conf.Api.PayloadSizeLimitInBytes()

		assert.Equalf(t, tc.sizeInBytes, sizeInBytes,
			"size %s should generate size in bytes %d", tc.size, tc.sizeInBytes)
		assert.NoErrorf(t, err, "size %s should return no error", tc.size)
	}
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
		conf := config.ApiConfig{PayloadSizeLimit: tc.size}

		_, err := conf.PayloadSizeLimitInBytes()
		if tc.shouldError {
			assert.Errorf(t, err, "size %s should result in error", tc.size)
		} else {
			assert.NoErrorf(t, err, "size %s should NOT result in error", tc.size)
		}
	}
}

func TestValidateCompressionTypesAndLevels(t *testing.T) {
	logTemplate := `
flows:
  - name: flow_1
    compression:
      type: "{{TYPE}}"
      level: "{{LEVEL}}"`

	testCases := []struct {
		compressionType  string
		compressionLevel string
		shouldError      bool
	}{
		{
			compressionType: "aaa",
			shouldError:     true,
		},
		{
			compressionType:  "123random",
			compressionLevel: "5",
			shouldError:      true,
		},
		{
			compressionType:  "gzip",
			compressionLevel: "10",
			shouldError:      true,
		},
		{
			compressionType:  "gzip",
			compressionLevel: "random",
			shouldError:      true,
		},
		{
			compressionType:  "gzip",
			compressionLevel: "0",
			shouldError:      true,
		},
		{
			compressionType:  "gzip",
			compressionLevel: "-1",
			shouldError:      true,
		},
		{
			compressionType:  "lzw",
			compressionLevel: "1", // lzw doesn't allow level on this project, but it is just ignored
			shouldError:      true,
		},
		{
			compressionType:  "gzip",
			compressionLevel: "9",
		},
		{
			compressionType:  "gzip",
			compressionLevel: "1",
		},
		{
			compressionType:  "gzip",
			compressionLevel: "3",
		},
		{
			compressionType:  "zlib",
			compressionLevel: "9",
		},
		{
			compressionType:  "deflate",
			compressionLevel: "9",
		},
		{
			compressionType:  "snappy",
			compressionLevel: "9",
		},
		{
			compressionType:  "zstd",
			compressionLevel: "9",
		},
	}

	for _, tc := range testCases {
		conf := strings.ReplaceAll(logTemplate, "{{TYPE}}", tc.compressionType)
		conf = strings.ReplaceAll(conf, "{{LEVEL}}", tc.compressionLevel)

		_, err := config.New([]byte(conf))

		if tc.shouldError {
			assert.Errorf(t, err, "type %s and level %s should return error on New",
				tc.compressionType, tc.compressionLevel)
		} else {
			assert.NoErrorf(t, err, "type %s and level %s should NOT return error on New",
				tc.compressionType, tc.compressionLevel)
		}
	}
}

func TestValidateFlowDecompressionTypes(t *testing.T) {
	logTemplate := `
flows:
  - name: flow_1
    ingestion:
      decompress:
        active: [{{TYPES}}]`

	testCases := []struct {
		types       []string
		shouldError bool
	}{
		{
			types:       []string{"aaa"},
			shouldError: true,
		},
		{
			types:       []string{"aaa", "something"},
			shouldError: true,
		},
		{
			types:       []string{"aaa", "gzip"},
			shouldError: true,
		},
		{
			types:       []string{""},
			shouldError: true,
		},
		{
			types:       []string(nil),
			shouldError: false,
		},
		{
			types:       []string{"gzip"},
			shouldError: false,
		},
		{
			types:       []string{"snappy"},
			shouldError: false,
		},
		{
			types:       []string{"zstd"},
			shouldError: false,
		},
		{
			types:       []string{"zlib"},
			shouldError: false,
		},
		{
			types:       []string{"deflate"},
			shouldError: false,
		},
		{
			types:       []string{"gzip", "zlib"},
			shouldError: false,
		},
		{
			types:       []string{"gzip", "snappy", "zlib", "zstd", "deflate"},
			shouldError: false,
		},
	}

	for _, tc := range testCases {
		var conf string
		if len(tc.types) == 0 {
			conf = strings.ReplaceAll(logTemplate, "{{TYPES}}", "")
		} else {
			types := strings.Join(tc.types, "\",\"")
			types = "\"" + types + "\""
			conf = strings.ReplaceAll(logTemplate, "{{TYPES}}", types)
		}

		_, err := config.New([]byte(conf))

		if tc.shouldError {
			assert.Errorf(t, err, "should error when decompression types are %v", tc.types)
		} else {
			assert.NoErrorf(t, err, "should NOT error when decompression types are %v", tc.types)
		}
	}
}

func TestErrorsIfMaxConcurrencyIsSetButActiveAlgorithmsIsNot(t *testing.T) {
	conf := `
flows:
  - name: flow_1
    ingestion:
      decompress:
        max_concurrency: 4`

	_, err := config.New([]byte(conf))

	assert.Errorf(t, err, "should error when max_concurrency is set but active is not (on ingestion option, inside flow)")
}

func TestErrorOnInvalidLogFormat(t *testing.T) {
	//TODO: implement me
}

func TestErrorWhenAccumulatorSizeIsZero(t *testing.T) {
	//TODO: implement me
}
