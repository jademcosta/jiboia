# Jiboia

Jiboia is an HTTP API that is able to move data into object storages for asynchronous processing.

An example of usage is if you want to ingest data into object storage but you want to do it using HTTP. You might decide to merge the data into larger chunks, which will increase the time it takes for data to be available but will reduce costs of the storage. Jiboia can do this, it is called accumulator!

It looks like a "pipeline" type of project, and it is! But it has less features, which allow it to be more performant.

You can define multiple "flows" of ingestion, and have a specific config for each one. The simplest config you can have is:

```yaml
flow:
  name: "my-flow"
  max_concurrent_uploads: 5
  external_queue:
    type: noop
  object_storage:
    type: s3
    config:
      bucket: my-bucket-1
      region: us-east-1
```

With this config you'll have 5 workers uploading data that is being sent. If the internal queues get full, Jiboia will deny more work until it has space again. Jiboia stores the data on memory, but if you configure it correctly it will be always uploading data, and this means data will sit for a very small time (less than a second) on memory.

The data will be able to be ingested on `http://localhost:9199/my-flow/async_ingestion` and will be sent into your bucket (considering that you have AWS keys configured on your machine).

You can have as many flows as you want, the only limitation is how much memopry you want Jiboia to operate with.

Right now it only support AWS S3 but the plan is to support more object storages in the future.

## Some of the features

Some features that will help you reuse Jiboia on multiple flows:
* Circuit breaker per flow - This will ensure no single flow can take Jiboia down.
* API key by flow - To avoid systems sending data into wrong paths.
* Accumulator - To merge small chunks of data into bigger ones, making it easier and cheaper to process. The accumulation can be by size (mandatory) and by time (optional). WHen accumulating, you can define a string that will be used as separator.
* Compression before uploading data to object storage - This will reduce the object storage costs and upload will be faster.
* It allows you to use an external queue to send a message after the data is uploaded into the storage.
* Jiboia prefixes the data being stored with time-related folders, so you can have better management of the data, in case you need to audit it.
* It has an optiuon to generate random prefixes so you can avoid being rate-limited by object storages services.
* Data is correctly flushed when Jiboia receives a SIGTERM, which means no data is lost (graceful shutdown).
* Top-tier observability, with several metrics for all its internals, split by flow.


## Using it

- [Download the latest release of Jiboia](https://github.com/jademcosta/jiboia/releases).
- Make working `config.yaml` file (see the [Configuration](#configuration) section below).
- Run it with `./jiboia --config config.yml`.

## Configuration

The best way to know all the available options for configuration is by reading the [config_example](https://github.com/jademcosta/jiboia/blob/main/config_example.yaml).

One important aspect of Jiboia is that it stores data in memory. This means that you need to give enough memory to have all internal queues filled. This can happen if, for example, the object storage starts to have a high latency for the upload. When this happens, the internal queues will probably fill, and if Jiboia doesn't have enough memory it will crash, which means data will be lost.

To avoid it, multiply the average package size (which can be get from the metrics) by the `in_memory_queue_max_size` config + `max_concurrent_uploads` config +  `queue_capacity` config from accumulator. But this is not an easy config. You have to take into account that if you are using an accumulator in a flow, the data that enters accumulator is smaller than the data leaving. So a single "slot of data" in the queue waiting to be consumed by accumulator uses less memory than the data that accumulator sends forward to be uploaded.

## Why Jiboia and not  <your prefered streaming/queueing project here>

Jiboia is a focused tool, built to deal with large volumes of data. It has a reduced number of features compared to other projects, and this allows it to be performant. So, when using it, it will probably be cheaper than alternatives. At the same time, it will probably have less features.

## Contributions

Read the `CONTRIBUTING` file.

## License

This project is licensed under the terms of the `LICENSE` file.
