---
title: "Managing nodes: settings"
---

Nodes are managed by the `node-manager` module (it is **enabled** by default).

## Parameters

* `instancePrefix` — the prefix to use when creating instances via the corresponding cloud provider module;
  * An optional parameter;
  * The default value can be calculated based on the `ClusterConfiguration` custom resource if the cluster is installed via the Deckhouse installer.
