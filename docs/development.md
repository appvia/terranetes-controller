## Development

A typical workflow for development would be to use Kind.

```shell
# Create a new local cluster for testing
$ kind create cluster

# Make a copy of the helm values (note ./dev is intentionally ignored by .gitignore)
$ mkdir -p dev
$ cp charts/values.yaml dev/values.yaml

# Use the following values
$ vim dev/values.yaml
```

```YAML
---
replicaCount: 1
controller:
  costs:
    # Add this if you're testing infracost
    #secret: infracost
  images:
    # is the controller image
    controller: quay.io/appvia/terraform-controller:latest
    # The terraform image used when running jobs
    executor: quay.io/appvia/terraform-executor:latest
```

```shell
# Builds the images locally as <IMAGE>:latest and loads them into a kind cluster
$ make controller-kind

# Change the values of the images to :latest in values.yaml
$ helm install terraform-controller charts --create-namespace --values dev/values.yaml
```

You can easily iterate locally by running `make controller-kind` again to build, load and restart.

### Running off the terminal

You can also run the controller locally connecting to a remote Kubernetes cluster. Here we are using kind again, but the controller will pick up whatever your current KUBE_CONFIG is defined as. Note, you'll need to apply the CRDs separately: `kubectl apply -f charts/crds`

```shell
# Create a new local cluster for testing
$ kind create cluster

$ make controller

# You can either use :latest, or if you need to change the executor image you can run the below.
# This will build the image and then loads it into the kind cluster
$ make executor-image-kind

# Run the controller locally; note we are turning off the webhooks here (haven't invested the time to figure out how to get that to work outside of a cluster).
$ bin/controller --enable-webhooks=false [flags]
```

### Updating the Kubernetes API Resources

If you need to update the API resources defined in `pkg/apis`, after updating the code simply run `make apis`; this will regenerate the deepcopies, registration and so forth.

### Running the tests

You can run the entire test suite via `make check`. All the tools "should" be in vendor, so no need to download anything.

### Running the E2E

Located in `test/e2e`, there is a group of bats tests which is used to check the full E2E. You need to have a kind cluster running, the aws cli and either have the `aws` secret configured for a provider in `terraform-system` already created or export AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY and AWS_REGION into the environment; you can take a look [here](test/e2e/integration/provider.bats).

To run the e2e: `BUCKET=<NAME_OF_S3_BUCKET> test/e2e/check-suite.sh`.

Please remove all the checks [here](e2e/test/integration).

### Components

The project is essentially made of these pieces:

* Controller which handles the reconciliation of the CRDs `(pkg/controller/{configuration, provider, policy})`.
* An API server (runs in the same process as the controller, though technically you could split out) used to stream the job logs from the central namespace back to developer namespaces `(pkg/apiserver)`.
* Admission and mutating webhooks (again runs inside the controller process) used to perform CRD validation and mutation of configurations `(pkg/handlers)`.
* The executor image `(image/Dockerfile.executor)`, this is how binaries are copied into job containers (i.e. terraform, infracost and checkov). Effectively if you need say `script.sh` to be available from a third party container, you can place into the executor image. On pod init the files froms `/assets` directory are copied into a shared emptyDir volume under `/run`. You can then call `/run/bin/<filename>` to utilizes the asset.
