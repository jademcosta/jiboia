# Jiboia project

Nothing here... yet :)


## Remember
* On SQS, The minimum message size is one character. The maximum size is 256 KB!


## S3 notification format
```javascript
{"Service":"Amazon S3",
"Event":"s3:TestEvent",
"Time":"2022-10-19T00:48:41.122Z",
"Bucket":"jiboia-testing-2",
"RequestId":"random-string",
"HostId":"random-string"}
```


```javascript
{"Records":[
  {
    "eventVersion":"2.1",
    "eventSource":"aws:s3",
    "awsRegion":"us-east-1",
    "eventTime":"2022-10-19T00:51:54.281Z",
    "eventName":"ObjectCreated:Put",
    "userIdentity":{"principalId":"Principal-id-here"},
    "requestParameters":{"sourceIPAddress":"IP-address"},
    "responseElements":{
      "x-amz-request-id":"random-string",
      "x-amz-id-2":"+random-string"
    },
    "s3":{
      "s3SchemaVersion":"1.0",
      "configurationId":"testing-the-eventz",
      "bucket":{
        "name":"jiboia-testing-2",
        "ownerIdentity":{"principalId":"Principal-id-here"},
        "arn":"arn:aws:s3:::jiboia-testing-2"
      },
      "object":{
        "key":"filename",
        "size":74,
        "eTag":"some-etag",
        "sequencer":"HEX-number-here"
      }
    }
  }
]}

```
