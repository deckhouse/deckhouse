---
title: Pod-reloader module
permalink: en/architecture/kubernetes-and-scheduling/pod-reloader.html
search: pod-reloader
description: Architecture of the pod-reloader module in Deckhouse Kubernetes Platform.
---

The [`pod-reloader`](/modules/pod-reloader/) module is based on [Reloader](https://github.com/stakater/Reloader). It provides the ability for automatic rollout on ConfigMap or Secret changes. Annotations are used for configuration. The module is running on **system** nodes.

For more details about module configuration and usage examples, refer to the [module documentation](/modules/pod-reloader/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`pod-reloader`](/modules/pod-reloader/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![Pod-reloader module architecture](../../images/architecture/kubernetes-and-scheduling/c4-l2-pod-reloader.png)

## Module components

The `pod-reloader` module consists of a single `pod-reloader` component that includes the following containers:

* **manager**: Main container.
* **kube-rbac-proxy**: Sidecar container with a Kubernetes RBAC-based authorization proxy that provides secure access to pod-reloader metrics. It is an [open-source project](https://github.com/brancz/kube-rbac-proxy).

## Module interactions

The module interacts with the `kube-apiserver` component:

* Watches ConfigMap and Secret resources.
* Triggers reload or rollout of pods.
* Authorizes requests for metrics.

The `prometheus-main` component interacts with the module by collecting module metrics.
