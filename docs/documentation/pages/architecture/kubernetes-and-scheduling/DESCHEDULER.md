---
title: Descheduler module
permalink: en/architecture/kubernetes-and-scheduling/descheduler.html
search: descheduler, rescheduling, balancing
description: Architecture of the descheduler module in Deckhouse Kubernetes Platform.
---

The [`descheduler`](/modules/descheduler/) module ensures operation of [Descheduler](https://github.com/kubernetes-sigs/descheduler) in Deckhouse Kubernetes Platform (DKP).

The module analyzes the cluster state periodically and evicts pods that match conditions defined in [active strategies](/modules/descheduler/#strategies). Evicted pods are then scheduled again according to the current cluster state. This helps redistribute workloads in line with the selected strategy.

If the [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md) service is registered in the cluster, the module uses it to obtain information about the current resource consumption.

Strategy parameters are configured using the [Descheduler](/modules/descheduler/cr.html#descheduler) custom resource. A module hook processes these resources and recreates configuration for Descheduler.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`descheduler`](/modules/descheduler/) module and its interactions with other DKP components are shown in the following diagram:

![Descheduler module architecture](../../images/architecture/kubernetes-and-scheduling/c4-l2-descheduler.svg)

## Module components

The `descheduler` module consists of a single **descheduler** component that includes the following containers:

* **descheduler**: Main container.
* **kube-rbac-proxy**: Sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to descheduler metrics.

## Module interactions

The module interacts with the **kube-apiserver** component:

* Watches standard Node and Pod resources.
* Retrieves current resource consumption through the [Metrics API](https://github.com/kubernetes/design-proposals-archive/blob/main/instrumentation/resource-metrics-api.md).
* Evicts running pods to match active strategies.
* Authorizes requests for metrics.

The **prometheus** component interacts with the module by collecting module metrics.
