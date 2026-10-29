---
title: Deckhouse Platform component registry
description: The container image registry from which the cluster pulls the images of Deckhouse Platform components.
lang: en
---

The registry is specified during cluster bootstrap in the [`deckhouse`](/modules/deckhouse/) parameter of [InitConfiguration](reference/api/cr.html#initconfiguration). How its configuration is managed in a running cluster depends on how the cluster is managed: in a [cluster fully managed by Deckhouse Platform](glossary.html#dp-managed-cluster), the [`registry`](/modules/registry/) module is used and the [registry management mode](glossary.html#registry-mode) is set in its [ModuleConfig](reference/api/cr.html#moduleconfig). In a Managed Kubernetes cluster, the `helper change-registry` is used.

For details, see [DP components registry](admin/configuration/registry/).
