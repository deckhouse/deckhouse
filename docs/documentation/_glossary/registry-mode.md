---
title: Registry management mode
description: The mode in which the registry module manages the pull paths of Deckhouse Platform component images, Unmanaged or Managed.
lang: en
---

The mode is set by the `mode` parameter in the [`registry`](/modules/registry/) [ModuleConfig](reference/api/cr.html#moduleconfig):

- `Unmanaged` (the default). The `registry` module does not manage the pull paths: the cluster pulls images from the [registry](glossary.html#component-registry) it was installed with.
- `Managed`. The `registry` module manages the pull paths. The `primary.upstream` parameter sets the upstream registry, and `storage.cache` enables the [in-cluster image cache](glossary.html#in-cluster-cache) on the master nodes. Without `primary.upstream`, the cluster is considered [isolated](glossary.html#air-gapped-cluster).

For details, see [Managing the registry in DP-managed clusters](admin/configuration/registry/managing-interaction.html).
