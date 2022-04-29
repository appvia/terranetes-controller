## Development

A typical workflow for development would be to use Kind.

```shell
# Create a new local cluster for testing
$ kind create cluster

# Make a copy of the helm values (note ./dev is intentionally ignored by .gitignore)
$ mkdir -p dev
$ cp charts/values.yaml dev/values.yaml

# Build the images locally and load into the kind cluster
$ make controller-kind

# Change the values of the images to :latest in values.yaml
$ helm install terraform-controller charts --create-namespace --values dev/values.yaml
```

You can easily iterate locally and perform `make controller-kind` to push the local images and reload
