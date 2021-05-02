---
title: "Managing control plane: configuration"
---

The `control-plane-manager` module is responsible for managing the cluster's control plane components. The cluster parameters that impact control plane management are derived from the `ClusterConfiguration` Custom Resource (it is created during the installation).

This module is **enabled** by default. You can disable it the standard way:

```yaml
controlPlaneManagerEnabled: "false"
```

## Parameters

<!-- SCHEMA -->
