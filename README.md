[![GPL license](https://img.shields.io/badge/License-GPL-blue.svg)](http://perso.crans.org/besson/LICENSE.html)
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/gomods/athens.svg)](https://github.com/gomods/athens)
[![GoReportCard example](https://goreportcard.com/badge/github.com/appvia/terranetes-controller)](https://goreportcard.com/report/github.com/appvia/terranetes-controller)
[![CodeQL](https://github.com/appvia/terranetes-controller/actions/workflows/codeql.yml/badge.svg)](https://github.com/appvia/terranetes-controller/actions/workflows/codeql.yml)

# **TERRANETES CONTROLLER**

The Terranetes Controller manages the life cycles of a terraform resource, allowing developers to self-serve dependencies in a controlled manner.

There's a number of open source controllers you could use that will run Terraform in Kubernetes. All of them deliver the developer outcome, however, none address the "consequences" of delivering that outcome in the first place; the risks, costs, security implications, the general lack of checks and balances. What sets the Terranetes controller apart is its opinion we first enable the platform team to deliver the outcome in a safe environment before it can be self-served by developers. Providing platform engineers with all the knobs, dials and levers to control and mitigate those risks is the key differentiator for Terranetes controller.

Documentation can be found at https://terranetes.appvia.io

![Architecture](docs/images/architecture.jpg)

**FEATURES**
---

### DEVELOPERS

- [Keep Terraform Configuration simple to use](https://terranetes.appvia.io/terranetes-controller/developer/configuration/)
- [Filter and write specific Terraform outputs to a secret](https://terranetes.appvia.io/terranetes-controller/developer/configuration/#connection-secret-reference)
- [View full Terraform log output](https://terranetes.appvia.io/terranetes-controller/developer/configuration/#viewing-the-changes)
- [Approve changes before application, supporting plan and apply workflows](https://terranetes.appvia.io/terranetes-controller/developer/configuration/#approving-a-plan)
- [See cost estimates prior to creating resources](https://terranetes.appvia.io/terranetes-controller/admin/costs/)
- [Support private terraform module sources](https://terranetes.appvia.io/terranetes-controller/developer/private/)
- [Directly reference FluxCD sources](https://terranetes.appvia.io/terranetes-controller/developer/flux/)
- [Drift Detection](https://terranetes.appvia.io/terranetes-controller/admin/drift/)

### PLATFORM ENGINEERS

- [Keep cloud credentials secure](https://terranetes.appvia.io/terranetes-controller/admin/providers/)
  - Restrict credentials provider use via namespace and label selectors
  - Don't expose credentials within a user's namespace
- [Define Guardrails around use](https://terranetes.appvia.io/terranetes-controller/admin/policy/)
  - Restrict to known module sources
  - Validate resource requests against Checkov policies
  - Automatically inject default configuration based on labels

**GETTING STARTED**
---

View the getting started guide at https://terranetes.appvia.io
