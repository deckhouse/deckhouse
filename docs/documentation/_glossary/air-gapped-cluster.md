---
title: Air-gapped cluster
description: A cluster without access to an external container image registry. The only source of images for it is the in-cluster cache.
lang: en
---

In the `Managed` [registry mode](glossary.html#registry-mode) the cluster is considered isolated when the `primary.upstream` parameter is not set. The [in-cluster cache](glossary.html#in-cluster-cache) is then filled with the `d8 mirror push` command and serves as the [registry of Deckhouse Platform components](glossary.html#component-registry).

For details, see [Managing the registry in DP-managed clusters](admin/configuration/registry/managing-interaction.html).
