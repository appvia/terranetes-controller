## Terraform Configuration CRD

A terraform module is represented by a `Configuration` CRD. Broken down this comprises of

**Module Reference** (spec.module)

The module reference dictates the module to run and uses the same format as `terraform` itself. For full details take a look at https://github.com/hashicorp/go-getter. But for quick reference

* SSH = git::ssh://git@example.com/foo/bar
* HTTPS = https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git?ref=v3.1.0

**Provider Reference** (spec.providerRef)

Is a reference (name and namespace) to the credentials which should be used to run the module. Note at present we don't wrap RBAC around the Providers, so a Configuration can specify Provider in it's or another namespace.

**Connection Secrets** (spec.writeConnectionSecretToRef)

The name of the secret *(inside the Configuration namespace)* where any module outputs from the terraform module are written as environment variables.

**Variables** (spec.variables)

Provides the ability to supply variables to the terraform module itself. These are converted to HCL and provided into the workflow via `-var-file` on the `plan` and `apply` stages.

```
apiVersion: terraform.appvia.io/v1alpha1
kind: Configuration
metadata:
  name: bucket
spec:
  # ssh git::ssh://git@example.com/foo/bar
  module: https://github.com/terraform-aws-modules/terraform-aws-s3-bucket.git?ref=v3.1.0

  providerRef:
    namespace: terraform-system
    name: default

  writeConnectionSecretToRef:
    name: test

  # An optional reference to a secret containing credentials to retrieve
  # the repository
  auth:
    name:

  variables:
    # -- The name of the bucket. If omitted, Terraform will assign a random, unique name
    bucket: rohith-test-1234
    # -- The canned ACL to apply
    acl: private
    # -- Map containing versioning configuration
    versioning:
      enabled: true
    # --Whether Amazon S3 should block public ACLs for this bucket
    block_public_acls: true
    # -- Whether Amazon S3 should block public bucket policies for this bucket
    block_public_policy: true
    # -- Whether Amazon S3 should ignore public ACLs for this bucket
    ignore_public_acls: true
    # -- Whether Amazon S3 should restrict public bucket policies for this bucket
    restrict_public_buckets: true
    # -- Map containing server-side encryption configuration
    server_side_encryption_configuration:
      rule:
        apply_server_side_encryption_by_default:
          sse_algorithm: "aws:kms"
        bucket_key_enabled: true
 ```

**Private Repositories**
---

If the repository is private, you can add SSH credentials via a secret and update the spec to reference the secret `spec.auth.name: ssh`.

```
$ kubectl -n apps create secret generic ssh --from-file=SSH_KEY_AUTH=id.rsa
```

You can also pass `GIT_USERNAME` and `GIT_PASSWORD` as an alternative to SSH.
