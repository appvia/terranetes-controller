## Development

A typical workflow for development would be to use Kind.

```shell
# Create a new local cluster for testing
$ kind create cluster

# Create a values file for helm (note ./dev is intentionally ignored by .gitignore)
$ mkdir -p dev

# Use the following values
$ vim dev/values.yaml
```

```YAML
controller:
  costs:
    # Add this if you're testing infracost
    #secret: infracost
  images:
    # is the controller image
    controller: ghcr.io/appvia/terranetes-controller:ci
    # The terraform image used when running jobs
    executor: ghcr.io/appvia/terranetes-executor:ci
```

```shell
# Builds the images locally as <IMAGE>:latest and loads them into a kind cluster
$ make controller-kind

# Change the values of the images to :latest in values.yaml
$ helm install terranetes-controller charts/terranetes-controller --create-namespace --values dev/values.yaml
```

You can easily iterate locally by running `make controller-kind` again to build, load and restart.

### Running off the terminal

You can also run the controller locally connecting to a remote Kubernetes cluster. Here we are using kind again, but the controller will pick up whatever your current KUBE_CONFIG is defined as. Note, you'll need to apply the CRDs separately: `kubectl apply -f charts/terranetes-controller/crds`

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

Located in `test/e2e`, there is a group of bats tests which is used to check the full E2E. You need to have a kind cluster running and a number of secrets in your environment.

The following secrets always required, regardless of cloud

* INFRACOST_API_KEY - containing the infracost cost api key from `infracost register`

For AWS i.e. `check-suite.sh --cloud aws` you will need the following environment variables.

* AWS_REGION
* AWS_ACCESS_KEY_ID
* AWS_SECRET_ACCESS_KEY

For Azure i.e `check-suite.sh --cloud azure`

* ARM_CLIENT_ID
* ARM_CLIENT_SECRET
* ARM_SUBSCRIPTION_ID
* ARM_TENANT_ID
* ARM_APP_ID
* ARM_APP_NAME

For Google

* GOOGLE_PROJECT
* GOOGLE_CREDENTIALS

You can use the following template

```shell
export INFRACOST_API_KEY=

# Required by AWS
export AWS_REGION=""
export AWS_ACCESS_KEY_ID=""
export AWS_SECRET_ACCESS_KEY=""

# Required by Azure (review terraform provider credentials for details on the fields)
export ARM_APP_ID=""
export ARM_APP_NAME=""
export ARM_CLIENT_ID=""
export ARM_CLIENT_SECRET=""
export ARM_SUBSCRIPTION_ID=""
export ARM_TENANT_ID=""

# Required by Google
export GOOGLE_PROJECT=""
# This will be a base64 of a Kubernetes secret which contains the GOOGLE_PROJECT and GOOGLE_CREDENTIALS
# environment variables. This is due to the JSON key file being a pain to handle i.e. the file would look like
# apiVersion: v1
# kind: Secret
# metadata:
#  name: google
# type: Opaque
# data:
#  GOOGLE_PROJECT: BASE64
#  GOOGLE_CREDENTIALS: BASE64
export GOOGLE_CREDENTIALS=BASE64 OF ABOVE FILE
```

1. Copy the above file and place into dev/credentials.sh _(note the dev/ folder is ignored by .gitignore)_.
2. Before running the E2E source the environment variables in via `source dev/credentials.sh`.
3. To run the e2e: `BUCKET=<NAME_OF_S3_BUCKET> test/e2e/check-suite.sh --cloud <aws|azure>`.

Please review all the checks [here](e2e/test/integration).

### Components

The project is essentially made of these pieces:

* Controller which handles the reconciliation of the CRDs `(pkg/controller/{configuration, provider, policy})`.
* An API server (runs in the same process as the controller, though technically you could split out) used to stream the job logs from the central namespace back to developer namespaces `(pkg/apiserver)`.
* Admission and mutating webhooks (again runs inside the controller process) used to perform CRD validation and mutation of configurations `(pkg/handlers)`.
* The executor image `(image/Dockerfile.executor)`, this is how binaries are copied into job containers (i.e. terraform, infracost and checkov). Effectively if you need say `script.sh` to be available from a third party container, you can place into the executor image. On pod init the files froms `/assets` directory are copied into a shared emptyDir volume under `/run`. You can then call `/run/bin/<filename>` to utilizes the asset.
