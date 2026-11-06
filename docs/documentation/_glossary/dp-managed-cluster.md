---
title: Cluster managed by Deckhouse Platform
description: A cluster in which Deckhouse Platform manages both the control plane and the nodes. The opposite is a Managed Kubernetes cluster, where the control plane is managed by a cloud provider.
lang: en
---

Deckhouse Platform can also be installed into an existing Managed Kubernetes cluster, such as GKE, EKS or AKS. In this case Deckhouse Platform does not manage the [control plane](glossary.html#control-plane), and some features are limited by the API of the cloud provider: for example, node management and the [`registry`](/modules/registry/) module are available only in clusters fully managed by Deckhouse Platform.

For details, see [Platform installation](installing/).
