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

## Ho to change retention policy?

At moment delete retention policy is hardcoded, and there is no way to change it [issue](https://github.com/deckhouse/deckhouse/issues/360)

## Why folder not deleted from server after cleanup?

If you do comething like `kubectl delete -f demo.yml` it does delete `LocalPathProvisioner` which is responsible for folder deletion, so in other words there is no one who will be able to run `rm -rf /mnt/kubernetes/demo` for you.

For folders to be cleanup, make sure do delete corresponding pods and then persistent volume claims, so provisioner will catch up and cleanup folders for you.
