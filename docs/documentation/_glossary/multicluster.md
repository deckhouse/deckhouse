---
title: Multicluster
description: An inter-cluster communication model that tightly connects clusters. The clusters share namespaces, and the services of one cluster are available to the others as local ones.
lang: en
---

Access to services from other clusters can be restricted by authorization policies. The model is implemented with Istio through the [`istio`](/modules/istio/) module and is built on the [service mesh](glossary.html#service-mesh). The model for independent clusters is [federation](glossary.html#federation).

For details, see [Inter-cluster cooperation](admin/configuration/network/alliance/).
