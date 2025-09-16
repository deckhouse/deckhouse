---
title: "YADRO Storage"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/external/yadro.html
d8Revision: ee
---

To manage volumes based on the [TATLIN.UNIFIED](https://yadro.com/ru/tatlin/unified) storage system,
you can use the `csi-yadro` module to create StorageClass resources through custom YadroStorageClass resources.

## Enable the module

To enable the `csi-yadro` module, run the following command:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-yadro
spec:
  enabled: true
  version: 1
EOF
```

Wait until `csi-yadro` is in the `Ready` status.
To check the status, run the following command:

```shell
d8 k get module csi-yadro -w
```

In the output, you should see information about the module:

```console
NAME        STAGE   SOURCE   PHASE       ENABLED   READY
csi-yadro                    Available   True      True
```

## Connect to the TATLIN.UNIFIED storage system

To connect to the TATLIN.UNIFIED storage system and enable configuring of StorageClass objects,
apply the following YadroStorageConnection resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroStorageConnection
metadata:
  name: yad1
spec:
  controlPlane:
    address: "172.19.28.184"
    username: "admin"
    password: "cGFzc3dvcmQ=" # Must be encoded in Base64
    ca: "base64encoded"
    skipCertificateValidation: true
  dataPlane:
    protocol: "iscsi"
    iscsi:
      volumeExportPort: "p50,p51,p60,p61"
EOF
```

## Create a StorageClass

To create a StorageClass, use the YadroStorageClass resource.
Creating a StorageClass resource manually without using YadroStorageClass can lead to errors.

Example command to create a StorageClass based on the TATLIN.UNIFIED storage system:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroStorageClass
metadata:
  name: yad1
spec:
  fsType: "xfs"
  pool: "pool-hdd"
  storageConnectionName: "yad1"
  reclaimPolicy: Delete
EOF
```

## Ensure the module works

To make sure the `csi-yadro` is working properly, check the pod status in the `d8-csi-yadro` namespace.
All pods must have the `Running` or `Completed` status.
The `csi-yadro` pods must be running on all nodes.

To check that the module works, run the following command:

```shell
d8 k -n d8-csi-yadro get pod -owide -w
```
