---
title: "The csi-nfs module"
description: "The csi-nfs module: General Concepts and Principles."
moduleStatus: experimental
---

This module provides CSI that manages volumes based on `NFS`. The module allows you to create a `StorageClass` in `Kubernetes` by creating [Kubernetes custom resources](./cr.html#nfsstorageclass) `NFSStorageClass`.

> **Caution!** The user is not allowed to create a `StorageClass` for the `nfs.csi.k8s.io` CSI driver.

## System requirements and recommendations

### Requirements
- Stock kernels shipped with the [supported distributions](/supported_versions.html#linux).
- Presence of a deployed and configured NFS server.

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling module

- Enable the `csi-nfs` module. This will result in the following actions across all cluster nodes:
    - registration of the CSI driver;
    - launch of service pods for the `csi-nfs` components.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
EOF
```

- Wait for the module to become `Ready`.

```shell
kubectl get module csi-nfs -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [NFSStorageClass](./cr.html#nfsstorageclass) resource. Here is an example command to create such a resource:

```yaml
kubectl apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: 10.223.187.3
    share: /
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

A directory `<directory from share>/<PV name>` will be created for each PV.

### Checking module health

You can verify the functionality of the module using the instructions [here](./faq.html#how-to-check-module-health)
