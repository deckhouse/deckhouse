---
title: "Deckhouse Kubernetes Platform module development"
permalink: en/module-development/
---

Deckhouse Kubernetes Platform (DKP) supports both built-in modules and modules that can be fetched from a _module source_. This section describes how to create and run a custom module in the cluster using the module source.

There are three key stages in the life of a module:


* **Development**: creating module code and its structure in the Git repository. The [**Module structure**](structure/) section describes the required components and the directories where they are located.
* **Building and publishing**: creating a module artifact and pushing it to the container registry. The [**Building and publishing**](build/) section describes where images are stored in the registry and what images are available.
* **Running in a cluster**: installing a module in a DKP cluster, enabling it, configuring it, and making sure it works as expected. The [**Running in a cluster**](module-development/run/) section describes how to run a configured module in a cluster.

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

The [Development and debugging](development/) section contains information about what tools can be used in module development and approaches to troubleshooting module errors. <!-- не факт -->

Join [the community](/community/), where you'll be sure to have your questions answered.
