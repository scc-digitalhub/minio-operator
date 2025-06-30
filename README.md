# MinIO Operator

![GitHub Actions Workflow Status](https://img.shields.io/github/actions/workflow/status/scc-digitalhub/minio-operator/release.yaml?event=release) [![license](https://img.shields.io/badge/license-Apache%202.0-blue)](https://github.com/scc-digitalhub/minio-operator/LICENSE) ![GitHub Release](https://img.shields.io/github/v/release/scc-digitalhub/minio-operator)
![Status](https://img.shields.io/badge/status-stable-gold)

A Kubernetes operator to handle instances of buckets, users and policies on [MinIO](https://min.io/).

Explore the full documentation at the [link](https://scc-digitalhub.github.io/docs/).

## Quick start

There is an available deployment file ready to be used. You can use it to install the operator and the CRD in your Kubernetes environment:

```sh
kubectl apply -f deployment.yaml
```

An example custom resource is found at `config/samples/minio_v1_*.yaml`. The CRDs included in the deployment file can be found at `config/crd/bases/` folder.

To launch a CR:

```sh
kubectl apply -f config/samples/minio_v1_bucket.yaml
kubectl apply -f config/samples/minio_v1_policy.yaml
kubectl apply -f config/samples/minio_v1_user.yaml
```

## Configuration

A number of environment variables must be configured. If you're using the `deployment.yaml` file, you will find them towards the end of the file.
```
WATCH_NAMESPACE: minio-operator-system
MINIO_ENDPOINT: 192.168.123.123:9000
MINIO_ACCESS_KEY_ID: minioadmin
MINIO_SECRET_ACCESS_KEY: minioadmin
MINIO_USE_SSL: false
MINIO_EMPTY_BUCKET_ON_DELETE: true
```
You can start from the provided "deployment.yaml" file and tailor it to your needs, e.g. modifying the resources that will be provided to CR containers.

### Custom Resource Properties

#### Bucket CR
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

#### Policy CR
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

#### User CR
A user's custom resource properties are:
- `accessKey`: **Required**.
- `secretKey`: **Required**.
- `accountStatus`: *Optional* (defaults to `enabled`). Either `enabled` or `disabled`.
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

## Development

The operator is developed with [Operator-SDK](https://sdk.operatorframework.io). Refer to its documentation and [tutorial](https://sdk.operatorframework.io/docs/building-operators/golang/tutorial/) for development details and commands. The [project layout](https://sdk.operatorframework.io/docs/overview/project-layout/) is also described there.

See CONTRIBUTING for contribution instructions.

## Security Policy

The current release is the supported version. Security fixes are released together with all other fixes in each new release.

If you discover a security vulnerability in this project, please do not open a public issue.

Instead, report it privately by emailing us at digitalhub@fbk.eu. Include as much detail as possible to help us understand and address the issue quickly and responsibly.

## Contributing

To report a bug or request a feature, please first check the existing issues to avoid duplicates. If none exist, open a new issue with a clear title and a detailed description, including any steps to reproduce if it's a bug.

To contribute code, start by forking the repository. Clone your fork locally and create a new branch for your changes. Make sure your commits follow the [Conventional Commits v1.0](https://www.conventionalcommits.org/en/v1.0.0/) specification to keep history readable and consistent.

Once your changes are ready, push your branch to your fork and open a pull request against the main branch. Be sure to include a summary of what you changed and why. If your pull request addresses an issue, mention it in the description (e.g., “Closes #123”).

Please note that new contributors may be asked to sign a Contributor License Agreement (CLA) before their pull requests can be merged. This helps us ensure compliance with open source licensing standards.

We appreciate contributions and help in improving the project!

## Authors

This project is developed and maintained by **DSLab – Fondazione Bruno Kessler**, with contributions from the open source community. A complete list of contributors is available in the project’s commit history and pull requests.

For questions or inquiries, please contact: [digitalhub@fbk.eu](mailto:digitalhub@fbk.eu)

## Copyright and license

Copyright © 2025 DSLab – Fondazione Bruno Kessler and individual contributors.

This project is licensed under the GNU Affero General Public License v3.0.
You may not use this file except in compliance with the License. Ownership of contributions remains with the original authors and is governed by the terms of the GNU Affero General Public License v3.0, including the requirement to grant a license to the project.
