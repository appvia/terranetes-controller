## Providers CRD

Providers are used to share the credentials down to consumers. At present we support

* `spec.source: secret` which references a kubernetes secret and mounts as environment variables into the executor.
* `spec.source: injected` which runs the executor with a defined service account. This is used to support pod identity or IRSA in AWS.

### Configuring IRSA for AWS

Update your helm values similar to the below. The import values here are the annotations being configured on the service account used by the executor.

```YAML
rbac:
  # Indicates we should create all the rbac
  create: true
  # service account for the controller
  controller:
    # Indicates we should provision the rbac
    create: true
    # annotations is a collection of annotations which should be added
    annotations: {}

  # Configuration for the terraform executor service account
  executor:
    # indicates we should create the terraform-executor service account
    create: true
    # annotations is a collection of annotations which should be added
    annotations:
      eks.amazonaws.com/role-arn: arn:aws:iam::<AWS_ACCOUNT_ID>:role/<NAME_OF_ROLE>
```

Now configure the provider to use an injected identity

```
$ cat <<EOF | kubectl -n terraform-system apply -f -
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Provider
metadata:
  name: default-irsa
spec:
  source: injected
  provider: aws
  serviceAccount: terraform-executor
EOF
```

You can now reference the provider as per normal.
