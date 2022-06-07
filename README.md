# Jiboia project


## Resources

### AWS
* https://docs.aws.amazon.com/pt_br/sdk-for-go/v1/developer-guide/setting-up.html
* https://docs.aws.amazon.com/pt_br/sdk-for-go/v1/developer-guide/configuring-sdk.html
### AWS S3
* https://docs.aws.amazon.com/pt_br/sdk-for-go/v1/developer-guide/using-s3-with-go-sdk.html
* https://docs.aws.amazon.com/pt_br/sdk-for-go/v1/developer-guide/s3-example-basic-bucket-operations.html
* https://docs.aws.amazon.com/sdk-for-go/api/service/s3/s3manager/#Uploader.Upload

### AWS SQS
* https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/sqs-example-create-queue.html
* https://docs.aws.amazon.com/sdk-for-go/api/service/sqs/


## Remember
* On SQS, The minimum message size is one character. The maximum size is 256 KB!


## S3 notification format
```javascript
{"Service":"Amazon S3",
"Event":"s3:TestEvent",
"Time":"2022-10-19T00:48:41.122Z",
"Bucket":"jiboia-testing-2",
"RequestId":"RX5X90XNK872686V",
"HostId":"0Sfvf93Q2fwvv5G5srEm2yl0XNf8WddwMFvmPW2EFJUDZTD4iS/Lt2SPgs7PHTH7o/3yrrlWA6w="}
```


```javascript
{"Records":[
  {
    "eventVersion":"2.1",
    "eventSource":"aws:s3",
    "awsRegion":"us-east-1",
    "eventTime":"2022-10-19T00:51:54.281Z",
    "eventName":"ObjectCreated:Put",
    "userIdentity":{"principalId":"A2632900H7RVAL"},
    "requestParameters":{"sourceIPAddress":"201.17.125.70"},
    "responseElements":{
      "x-amz-request-id":"8NXBV166FTVWXCZM",
      "x-amz-id-2":"+EoLl9/OuHsWmPf70T5OW8ai4dVMrk7PqkWXGPY+k93UNqafqnZ1fHXufJ80WGWlZHhb5ICDhPVhUoIeJCZuzk1JgylWVDCx"
    },
    "s3":{
      "s3SchemaVersion":"1.0",
      "configurationId":"testing-the-eventz",
      "bucket":{
        "name":"jiboia-testing-2",
        "ownerIdentity":{"principalId":"A2632900H7RVAL"},
        "arn":"arn:aws:s3:::jiboia-testing-2"
      },
      "object":{
        "key":"raft-in-go.txt",
        "size":74,
        "eTag":"36ca1775fe8007ff6286bf43e024c9d1",
        "sequencer":"00634F4A2A414C9AF6"
      }
    }
  }
]}

```
