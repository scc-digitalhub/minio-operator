# MinIO Operator
A Kubernetes operator to handle instances of buckets, users and policies on [MinIO](https://min.io/).

## Installation
A number of environment variables must be configured. If you're using the `deployment.yaml` file, you will find them towards the end of the file.
```
WATCH_NAMESPACE: minio-operator-system
MINIO_ENDPOINT: 192.168.123.123:9000
MINIO_ACCESS_KEY_ID: minioadmin
MINIO_SECRET_ACCESS_KEY: minioadmin
MINIO_USE_SSL: false
MINIO_EMPTY_BUCKET_ON_DELETE: true
```

Install operator and CRD:
```sh
kubectl apply -f deployment.yaml
```

Example CRs can be found at `config/samples/minio_v1_*.yaml`. Apply them with:
```sh
kubectl apply -f config/samples/minio_v1_bucket.yaml
kubectl apply -f config/samples/minio_v1_policy.yaml
kubectl apply -f config/samples/minio_v1_user.yaml
```

## Bucket CR
A bucket's custom resource properties are:
- `name`: **Required**.
- `quota`: *Optional*. Number in bytes.

A valid sample spec configuration is:
``` yaml
...
spec:
  name: my-bucket
  quota: 10000000
```

## Policy CR
A policy's custom resource properties are:
- `name`: **Required**.
- `content`: **Required**. Multi-line JSON string of the policy's contents.

A valid sample spec configuration is:
``` yaml
...
spec: 
  name: my-policy
  content: >-
    {
      "Version": "2012-10-17",
      "Statement": [
        {
          "Effect": "Allow",
          "Action": [
            "s3:GetBucketLocation",
            "s3:GetObject"
          ],
          "Resource": [
            "arn:aws:s3:::*"
          ]
        }
      ]
    }
```

## User CR
A user's custom resource properties are:
- `accessKey`: **Required**.
- `secretKey`: **Required**.
- `policies`: *Optional*. List of policy names.

A valid sample spec configuration is:
``` yaml
...
spec:
  accessKey: usertest
  secretKey: usertest
  policies:
    - readonly
    - diagnostics
    - my-policy
```
