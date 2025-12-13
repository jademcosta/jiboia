package app

const confYaml = `
o11y:
  log:
    level: error
    format: json

api:
  port: 9099
  payload_size_limit: 120

flows:
  - name: "int_flow"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 2
    timeout: 120
    accumulator:
      size: 20
      separator: "_n_"
      queue_capacity: 10
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-flow1.com"
  - name: "int_flow2"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: localstorage
        config:
          path: "/tmp/int_test2"
  - name: "int_flow3"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    ingestion:
      token: "some secure token"
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-27836178236.com"
  - name: "int_flow4"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 2
    timeout: 120
    ingestion:
      decompress:
        active: ['gzip', 'snappy']
    accumulator:
      size: 20
      separator: ""
      queue_capacity: 10
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-flow4.com"
  - name: "int_flow5"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    ingestion:
      decompress:
        active: ['gzip', 'snappy']
    accumulator:
      size: 19
      separator: ""
      queue_capacity: 10
    external_queues:
      - type: noop
        config: ""
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-flow5-1.com"
      - type: httpstorage
        config:
          url: "http://non-existent-flow5-2.com"
`

const confForCompressionYaml = `
o11y:
  log:
    level: error
    format: json

api:
  port: 9099

flows:
  - name: "int_flow3"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    ingestion:
      token: "some secure token"
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-3.com"
  - name: "int_flow4"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    compression:
      type: gzip
      level: "5"
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-4.com"
`

const confForCBYaml = `
o11y:
  log:
    level: error
    format: json

api:
  port: 9098

flows:
  - name: "cb_flow"
    in_memory_queue_max_size: 2
    max_concurrent_uploads: 3
    timeout: 120
    accumulator:
      size: 5
      separator: "_n_"
      queue_capacity: 2
    external_queues:
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "{{OBJ_STORAGE_URL}}"
`

const conf4HighConcurrencyYaml = `
o11y:
  log:
    level: warn
    format: json

api:
  port: 9099
  payload_size_limit: 120

flows:
  - name: "int_flow5"
    in_memory_queue_max_size: 10000
    max_concurrent_uploads: 3
    timeout: 120
    ingestion:
      decompress:
        active: ['gzip', 'snappy']
    accumulator:
      size: 19
      separator: ""
      queue_capacity: 10000
    external_queues:
      - type: noop
        config: ""
      - type: noop
        config: ""
    object_storages:
      - type: httpstorage
        config:
          url: "http://non-existent-flow5-1.com"
      - type: httpstorage
        config:
          url: "http://non-existent-flow5-2.com"
`
