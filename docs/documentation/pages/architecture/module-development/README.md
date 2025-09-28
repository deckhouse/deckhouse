---
title: "Deckhouse Kubernetes Platform module development"
permalink: en/architecture/module-development/
lang: en
---

Deckhouse Kubernetes Platform (DKP) supports both built-in modules and modules that can be fetched from a [module source](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#modulesource). This section details what the DKP module is and how it works.

Creating a module consists of the following stages:

* **Development**: creating module code and its structure in the Git repository. The [**Module structure**](structure/) section outlines which components there are and in which directories they are located.
* **Building**: creating a module artifact and pushing it to the container registry. The [**Building and publishing**](build/) section describes where images are stored in the registry and at what paths they are available.
* **Running in a cluster**: delivering the module to a cluster managed by the DKP. The [**Running in a cluster**](run/) section describes how to activate the module, configure its parameters, and test its functionality (including handling CRDs and troubleshooting).
* **Dependencies**: configuring module dependencies, including DKP versions, Kubernetes, and other critical components. This stage is covered in the [**Module dependencies**](dependencies/) section.

## Requirements

You will need the following tools to develop DKP modules:

* [git](https://git-scm.com) — version control system;
* [sed](https://github.com/mirror/sed) — stream editor;
* [yq](https://github.com/mikefarah/yq) — CLI tool for processing data in JSON, YAML, and XML formats;
* [jq](https://jqlang.github.io/jq/) — CLI tool for processing data in JSON, YAML, and XML formats;
* [werf](https://werf.io/) — (optional) CLI tool for building images. You will need it for building module artifacts locally;
* [crane](https://github.com/google/go-containerregistry/tree/main/cmd/crane#crane) — (optional) CLI tool for working with the container registry. You may need it [when debugging](development/).

The container registry where [module artifacts](build/) will be stored must support a nested repository structure. A registry such as [Docker Registry v2](https://github.com/distribution/distribution) or [Harbor](https://goharbor.io/) is a good choice.

## Before you start

To get an idea of how DKP modules work, check out [addon-operator](https://github.com/flant/addon-operator) and [shell-operator](https://github.com/flant/shell-operator).

* Review the operator documentation on the concept of hooks, e.g., [what a hook configuration is and what functions it provides](https://flant.github.io/shell-operator/HOOKS.html#hook-configuration). The configuration is used to configure the data that will be available from the hook.
* Check out [bindings](https://flant.github.io/addon-operator/HOOKS.html#bindings). Bindings are events that trigger the hook. They are specified in the hook configuration. A hook can be triggered not only by Kubernetes events, but also, e.g., on a schedule or before a module is started.
> The Hook allows you to keep values in memory and use them later when rendering Helm templates. We recommend reading the [Hooks and Helm values](https://flant.github.io/addon-operator/OVERVIEW.html#hooks-and-helm-values) section to learn more about this feature as well as the module's operating cycle.
* Explore [the concept of snapshots](https://flant.github.io/shell-operator/HOOKS.html#snapshots). With snapshots, you can implement a reconciliation loop approach that is more efficient than event subscription.
 > This is how DKP implements support for all existing backend module hooks.
* Additionally, hooks can be used instead of the Prometheus exporter. Hooks can provide metrics that DKP will export. See [metrics](https://flant.github.io/addon-operator/metrics/METRICS_FROM_HOOKS.html#custom-metrics).

## Got a question?

The [Development and debugging](development/) section contains information about what tools can be used in module development and approaches to troubleshooting module errors.

Join [the community](/community/), where you'll be sure to have your questions answered.
