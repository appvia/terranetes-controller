<p align="right">
  <img src="docs/images/cogs.png" width="130">
<p>

[![GPL license](https://img.shields.io/badge/License-GPL-blue.svg)](http://perso.crans.org/besson/LICENSE.html) [![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/gomods/athens.svg)](https://github.com/gomods/athens) [![GoReportCard example](https://goreportcard.com/badge/github.com/appvia/terraform-controller)](https://goreportcard.com/report/github.com/appvia/terraform-controller)

# **TERRAFORM CONTROLLER**

Terraform Controller manages the life cycles of a terraform resource, allowing developers to self-serve dependencies in a controlled manner.

**DEVELOPERS**

* Workflows are run outside developer namespace so credentials can be shared without being exposed.
* Changes can be approved beforehand, following a plan and apply workflow.
* Developers can view and debug the terraform workflows from their our namespaces.

**PLATFORM ENGINEERS**

* Place guardrails around which modules can be consumed.
* Automatically inject environment specific configuration (dev, prod, cost centres and so forth) into the module, based on labels.
* Allow developers to see the associated costs to their configurations


**ROADMAP**
---

- Ability to set and control budgets
- Integration of policy via [checkov](https://www.checkov.io/)

**GETTING STARTED**
---

#### Prerequisites

* Helm CLI (https://helm.sh/docs/intro/install/
* Kind (https://kind.sigs.k8s.io/)

The quickest way to get up the running is via the Helm chart.

```shell
$ git clone git@github.com:appvia/terraform-controller.git
$ cd terraform-controller
# kind create cluster
$ helm install -n terraform-system terraform-controller charts/ --create-namespace
$ kubectl -n terraform-system get po

```

* Configure credentials for developers

```shell
# The following assumes you can using static credentials, for managed pod identity see docs

$ kubectl -n terraform-system create secret generic aws \
  --from-literal=AWS_ACCESS_KEY_ID=<ID> \
  --from-literal=AWS_SECRET_ACCESS_KEY=<SECRET> \
  --from-literal=AWS_REGION=<REGION>
$ kubectl -n terraform-system apply -f examples/provider.yaml
$ kubectl -n terraform-system get provider -o yaml
```

* Create your first configuration

```shell
$ cat examples/configuration.yaml # demos a s3 bucket
$ kubectl create namespace apps

# NOTE: Make sure to change the bucket name in examples/configuration.yaml
# spec.variables.bucket
$ vim examples/configuration.yaml
$ kubectl -n apps apply -f examples/configuration.yaml
$ kubectl -n apps get po

# Straight away a job is created to 'watch' the terraform workflow
$ kubectl -n apps logs -f <POD_ID>

# Check the module output
$ kubectl -n apps get secret test -o yaml
```

* Approve the plan

By default unless the `spec.enableAutoApproval` is true, all changes must be approved before acting on. An annotation is used to approve the previous plan.

```shell
$ kubectl -n apps annotate configurations.terraform.appvia.io bucket "terraform.appvia.io/apply"=true --overwrite
```

**ADDITIONAL RESOURCES**

- You can review the specification of the terraform configurations [here](docs/crd.md)
