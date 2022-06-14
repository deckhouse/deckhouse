---
title: "The local-path-provisioner module: FAQ"
---

## How to configure Prometheus to use local storage for storing data?

Deploy CR `LocalPathProvisioner`:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
```

- `spec.nodeGroups` must match node group where prometheus pods run.
- `spec.path` - node data path.

Add to the Deckhouse configuration (configMap `d8-system/deckhouse`):

```yaml
prometheus: |
  longtermStorageClass: localpath-system
  storageClass: localpath-system
```

Wait for the restart of Prometheus Pods.
