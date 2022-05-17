## Terraform Controller

### How does the controller work?

1. Watches for Configuration CRD in all namespaces.
2. Renders a batch job from the Configuration CRD, options defined on command line and Policy CRDs.
3. Retrieves the credentials from a provider.
4. Configures a batch job in the controller namespace to run terraform job.
5. Creates a corresponding job in the developer namespace which is used watch the logs from the running job.

### Configuring Providers

Providers are used to share the credentials down to consumers. At present we support

* `spec.source: secret` which references a kubernetes secret and mounts as environment variables into the executor.
* `spec.source: injected` which runs the executor with a defined service account. This is used to support pod identity or IRSA in AWS.

#### Configuring IRSA (Pod Identity) for AWS

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

### Costs Integration

The costs integration allows developers to see their associated costs before applying the terraform. This controller currently uses [infracost](https://infracost.io) to extract the predicted costs of a configuration, exposing the cost on the kubernetes status and viewable via `$ kubectl get configuration`

You can configure the integration via first

1. Create a secret containing the Infracost API token.

`$ kubectl -n terraform-system create secret generic infracost --from-literal=INFRACOST_API_KEY=$INFRACOST_API_KEY`

2. Configure the controller to enable infracost by updating the controller flag.
```YAML
controller:
  costs:
    # is the name of the secret you created in the controller namespace above
    name: infracost
```

3. Update the helm chart

`$ helm upgrade terraform-controller charts/`

### Module Policy

You can control the source of the terraform modules permitted to run by creating a [Policy](charts/crds/terraform.appvia.io_policies.yaml). The following policy enforces globally only modules the `appvia` organization are permitted.

```YAML
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Policy
metadata:
  name: permitted-modules
spec:
  constraints:
    modules:
      allowed:
        - "https://github.com/appvia/.*"
```

### Default Environment Variables

Default environments provides the ability to inject environment specific variables into a configuration based on a selector. Example might be

* Adding environment specific options, vpc id, tags etc. Elements which you don't want developers to deal with.
* Adding project specific tags - i.e. costs code, project id and so forth.

You can configure via a Policy, an example below

```YAML
---
apiVersion: terraform.appvia.io/v1alpha1
kind: Policy
metadata:
  name: environment-defaults
spec:
  defaults:
    - selector:
        namespace:
          matchExpressions:
            - key: kubernetes.io/metadata.name
              operator: Exists
      variables:
        environment: dev
```

## Configure a Job Template

When a configuration is changed i.e. up for plan, apply or destroy, the controller uses a template to render the final job configuration. Taking the options provided on the controller command, custom policies and the terraform configuration itself, a batch job is created to execute the change. The default template for this can be found [here](https://github.com/appvia/terraform-controller/blob/master/pkg/assets/job.yaml.tpl).

### Overloading the template

While not required in the vast majority of cases this template can be overridden allowing platform engineers to customize the pipeline. You might want to;

* Add a notification on failed configuration, or send a notification when a configuration fails policy.
* Add a new feature into the pipeline such as swapping out the default [checkov](https://www.checkov.io) for another policy engine.

You can change the template via uploading a configmap into the namespace where the controller lives;

```shell
# create the template configmap (note the key name of job.yaml)
$ kubectl -n terraform-system create configmap template --from-file=job.yaml=<PATH>

# update the helm values to override the template
controller:
  templates:
    job: <NAME_OF_CONFIG_MAP>
```
