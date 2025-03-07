---
title: "HPE storage"
permalink: en/storage/admin/external/hpe.html
---

This section installs and configures the CSI driver for HPE SAN. The module allows you to create a `StorageClass` in `Kubernetes` by creating [Kubernetes custom resources](./cr.html#yadrostorageclass) `YadroStorageClass`.

> **Caution!** The user is not allowed to create a `StorageClass` for the `csi.hpe.com` CSI driver.
> **Caution!** Currently, supports 3par SAN devices. For other HPE SAN support please contact tech support.

## System requirements and recommendations

### Requirements

- Presence of a deployed and configured HPE SAN.
- Unique iqn in /etc/iscsi/initiatorname.iscsi on each of Kubernetes Nodes

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling module

- Enable the `csi-hpe` module. This will result in the following actions across all cluster nodes:
  - registration of the CSI driver;
  - launch of service pods for the `csi-hpe` components.

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-hpe
spec:
  enabled: true
  version: 1
EOF
```

- Wait for the module to become `Ready`.

```shell
kubectl get module csi-hpe -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [HPEStorageClass](./cr.html#hpestorageclass) and [HPEStorageConnection](./cr.html#hpestorageconnection) resource. Here is an example command to create such a resource:

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HPEStorageConnection
metadata:
  name: hpe
spec:
  controlPlane:
    backendAddress: "172.17.1.55" # mutable, SAN API address
    username: "3paradm" # mutable, API username
    password: "3pardata" # mutable, API password
    serviceName: "primera3par-csp-svc"
    servicePort: "8080"
EOF
```

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: HPEStorageClass
metadata:
  name: hpe
spec:
  pool: "test-cpg"
  accessProtocol: "iscsi" # fc or iscsi (default iscsi), immutable
  fsType: "xfs" # xfs, ext3, ext4, btrfs (default ext4), mutable
  storageConnectionName: "hpe" # immutable
  reclaimPolicy: Delete # Delete of Retain
  cpg: "test-cpg"
EOF
```

- You can check objects creation (Phase must be `Created`):

```shell
d8 k get hpestorageconnections.storage.deckhouse.io <hpestorageconnection name>
```

```shell
d8 k get hpestorageclasses.storage.deckhouse.io <hpestorageclass name>
```

### How to check module health?

To do this, you need to check the status of the pods in the `d8-csi-hpe` namespace. All pods should be in the `Running` or `Completed` state and should be running on all nodes.

```shell
d8 k -n d8-csi-hpe get pod -owide -w
```
