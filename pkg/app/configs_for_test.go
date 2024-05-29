package app

const confYaml = `
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
      size_in_bytes: 20
      separator: "_n_"
      queue_capacity: 10
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
      config:
        url: "http://non-existent-flow1.com"
  - name: "int_flow2"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: localstorage
      config:
        path: "/tmp/int_test2"
  - name: "int_flow3"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    ingestion:
      token: "some secure token"
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
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
      size_in_bytes: 20
      separator: ""
      queue_capacity: 10
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
      config:
        url: "http://non-existent-flow4.com"
`

const confForCompressionYaml = `
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
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
      config:
        url: "http://non-existent-3.com"
  - name: "int_flow4"
    in_memory_queue_max_size: 4
    max_concurrent_uploads: 3
    timeout: 120
    compression:
      type: gzip
      level: "5"
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
      config:
        url: "http://non-existent-4.com"
`

const confForCBYaml = `
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
      size_in_bytes: 5
      separator: "_n_"
      queue_capacity: 2
    external_queue:
      type: noop
      config: ""
    object_storage:
      type: httpstorage
      config:
        url: "{{OBJ_STORAGE_URL}}"
`
