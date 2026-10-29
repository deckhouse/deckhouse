---
title: Federation
description: An inter-cluster communication model that connects independent clusters. Each cluster keeps its own namespaces and services, and access between clusters is configured explicitly.
lang: en
---

In a federation, the administrator defines which services are accessible from outside the cluster. The model is implemented with Istio through the [`istio`](/modules/istio/) module and is built on the [service mesh](glossary.html#service-mesh). The model for tightly connected clusters is [multicluster](glossary.html#multicluster).

For details, see [Inter-cluster cooperation](admin/configuration/network/alliance/).
