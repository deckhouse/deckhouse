---
title: Adaptive-resource-management module
permalink: en/architecture/kubernetes-and-scheduling/adaptive-resource-management.html
search: adaptive-resource-management, autovpa
description: Architecture of the adaptive-resource-management module in Deckhouse Kubernetes Platform.
---

The [`adaptive-resource-management`](/modules/adaptive-resource-management/) module  is intended for cluster administrators and platform engineers who want to automate the selection of resource requests and limits for workloads. It eliminates the need to manually tune CPU and memory parameters by leveraging VPA (Vertical Pod Autoscaler) recommendations.

The module deploys the AutoVPA controller, based on [Goldilocks](https://github.com/FairwindsOps/goldilocks) with additional Deckhouse-specific enhancements, which automatically creates and maintains VPA objects for workloads in selected namespaces and provides resource recommendations. AutoVPA replaces one-off, per-application guesswork with evidence-based sizing across the whole fleet: it prevents over-provisioning that drives up cloud spend, gives workloads reliable performance guardrails by basing requests and limits on observed usage rather than estimates, and scales the same recommendation workflow to as many workloads and namespaces as you manage — without touching a single manifest.

Main features:

* Automatic creation of VPA objects for Deployments, StatefulSets, DaemonSets, and Jobs in managed namespaces.
* Flexible namespace selection: manage all namespaces, only namespaces with a specific label, or namespaces matching a label selector.
* Combined selection mode: use a label selector together with the special label `autovpa.deckhouse.io/enabled` to form a union of matching namespaces.
* For the default recommendation-only mode, no changes to existing workload manifests are required; VPA objects are created and maintained automatically.
* Per-workload and per-namespace VPA tuning via `autovpa.deckhouse.io/*` annotations and labels.
* Minimal resource footprint: the controller runs as a single replica with low CPU and memory requirements.

For more details about module configuration and usage examples, refer to the [module documentation](/modules/adaptive-resource-management/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`adaptive-resource-management`](/modules/adaptive-resource-management/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![Adaptive-resource-management module architecture](../../images/architecture/kubernetes-and-scheduling/c4-l2-adaptive-resource-management.png)

## Module components

The `adaptive-resource-management` module consists of a single **autovpa-controller** component that includes the following container:

* **autovpa**: Main container.

## Module interactions

The module interacts with the **kube-apiserver** component:

* Creates VPA objects for workloads.
