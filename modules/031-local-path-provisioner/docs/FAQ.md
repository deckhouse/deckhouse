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

## What happens to existing volumes when `spec.path` changes?

New volumes are created under the new path. Volumes created earlier stay in the old directory. With `reclaimPolicy: Delete`, a helper Pod removes the directory of a deleted volume and mounts the old path to do so. When Pod Security Standards in system namespaces reject Pods instead of only warning about them, the SecurityPolicyException `helper-<name>` of the LocalPathProvisioner allows only the current path, so the helper Pod for an old volume is rejected: the PersistentVolume stays `Released`, and its data stays on the node. With `reclaimPolicy: Retain` (the default), no helper Pod is started on deletion, and this limitation does not apply.

Before changing `spec.path`, delete the PersistentVolumeClaims of the volumes created under the old path. If such a volume stays `Released` after the change, delete its directory on the node and then the PersistentVolume.
