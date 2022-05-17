## Executor Image

At present an executor image found [here](https://github.com/appvia/terraform-controller/blob/master/images/Dockerfile.executor) is used to run the terraform jobs. You can of course build your own image and update the controller flag `--executor-image`. This can also be update via the helm values

```YAML
controller:
  images:
    executor: <DOCKER_IMAGE>
```

**Note** with the introduction of [v0.1.0](https://github.com/appvia/terraform-controller/pull/10) we are deprecating the executor image and instead allowing the platform admin to define which images they want to use themselves
