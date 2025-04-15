---
title: "Local Path Provisioner Storage"
permalink: en/admin/storage/sds/local-path-provisioner.html
lang: en
---

Deckhouse Kubernetes Platform provides the ability to configure local storage using Local Path Provisioner. This is a simple solution without support for snapshots or volume size limits, best suited for development, testing, and small clusters. It enables the use of local disk space on Kubernetes nodes to create PersistentVolumes without relying on external storage systems.

## How it works

For each [LocalPathProvisioner](../../reference/cr/localpathprovisioner/) resource, a corresponding `StorageClass` object is created.

The set of nodes eligible for the `StorageClass` is determined based on the `nodeGroups` field from the custom resource. This topology is used for pod scheduling.

When a pod requests a volume:
- a `HostPath` PersistentVolume is created;
- a directory is created on the target node with a path composed of the `path` parameter, the PV name, and the PVC name.

Example path:

```shell
/opt/local-path-provisioner/pvc-d9bd3878-f710-417b-a4b3-38811aa8aac1_d8-monitoring_prometheus-main-db-prometheus-main-0
```

## Limitations

- It is not possible to set a size limit for created volumes.

## Example LocalPathProvisioner resources

### ReclaimPolicy: Retain (default)

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

### ReclaimPolicy: Delete

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: LocalPathProvisioner
metadata:
  name: localpath-system
spec:
  nodeGroups:
  - system
  path: "/opt/local-path-provisioner"
  reclaimPolicy: "Delete"
```

## Configuring Prometheus to use local storage

1. Apply the [LocalPathProvisioner](../../reference/cr/localpathprovisioner/) resource:

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

1. Ensure that `spec.nodeGroups` matches the NodeGroup where Prometheus will be running.

1. Specify the name of the created StorageClass in the Prometheus configuration:

```yaml
longtermStorageClass: localpath-system
storageClass: localpath-system
```

1. Wait for Prometheus pods to restart.
