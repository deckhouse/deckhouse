---
title: "Managing nodes: configuration"
---

This module is **enabled** by default. You can disable it the standard way.

```yaml
nodeManagerEnabled: "false"
```

## Parameters

* `instancePrefix` — the prefix to use when creating instances via the corresponding cloud provider module;
  * An optional parameter;
  * The default value can be calculated based on the `ClusterConfiguration` custom resource if the cluster is installed via the Deckhouse installer.
