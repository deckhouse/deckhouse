---
title: "The local-path-provisioner module: FAQ"
---

## How to configure Prometheus to use local storage for storing data?

Deploy custom resource `LocalPathProvisioner`:

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

- `spec.nodeGroups` must match NodeGroup where prometheus Pods run.
- `spec.path` - node data path.

Add to the following parameters to the `prometheus` module configuration:

```yaml
longtermStorageClass: localpath-system
storageClass: localpath-system
```

Wait for the restart of Prometheus Pods.
