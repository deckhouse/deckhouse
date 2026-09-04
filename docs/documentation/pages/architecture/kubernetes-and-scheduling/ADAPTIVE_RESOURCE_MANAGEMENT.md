---
title: Adaptive-resource-management module
permalink: en/architecture/kubernetes-and-scheduling/adaptive-resource-management.html
search: adaptive-resource-management, autovpa
description: Architecture of the adaptive-resource-management module in Deckhouse Kubernetes Platform.
---

The [`adaptive-resource-management`](/modules/adaptive-resource-management/) module  lets you automate the selection of resource requests and limits for workloads by leveraging the Vertical Pod Autoscaler (VPA) recommendations.

The module deploys the **AutoVPA** controller based on [Goldilocks](https://github.com/FairwindsOps/goldilocks) tailored for Deckhouse Kubernetes Platform (DKP). The controller automatically creates and maintains VPA objects for workloads in selected namespaces and provides recommendations on resource configuration.

Main features:

* Automatic creation of VPA objects for Deployments, StatefulSets, DaemonSets, and Jobs in managed namespaces.
* Flexible namespace selection: manage all namespaces, only namespaces with a specific label, or namespaces matching a label selector.
* Combined namespace selection mode using a label selector together with the special label `autovpa.deckhouse.io/enabled`.
* Automatic creation of VPA objects in recommendation-only mode without changing the workload manifests.
* Per-workload and per-namespace VPA tuning via `autovpa.deckhouse.io/*` annotations and labels.
* Minimal resource footprint: the controller runs as a single replica and requires low CPU and memory resources.

For more details about the module configuration and usage examples, refer to the [module documentation](/modules/adaptive-resource-management/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`adaptive-resource-management`](/modules/adaptive-resource-management/) module and its interactions with other components of DKP are shown in the following diagram:

![Adaptive-resource-management module architecture](../../images/architecture/kubernetes-and-scheduling/c4-l2-adaptive-resource-management.png)

## Module components

The `adaptive-resource-management` module consists of a single **autovpa-controller** component that includes **autovpa**, the main container.

## Module interactions

The module interacts with the **kube-apiserver** component for creating VPA objects for workloads.
