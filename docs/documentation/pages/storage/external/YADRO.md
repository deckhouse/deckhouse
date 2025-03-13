---
title: "TATLIN.UNIFIED (Yadro) unified storage"
permalink: en/storage/admin/external/yadro.html
d8Revision: ee
---

Deckhouse supports integration with the [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) storage system, enabling volume management in Kubernetes. This allows the use of centralized storage for containerized workloads, ensuring high performance and fault tolerance.

This page provides instructions on connecting TATLIN.UNIFIED (Yadro) to Deckhouse, configuring the connection, creating a StorageClass, and verifying system functionality.

## Enabling the module

To manage volumes based on the [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) storage system in Deckhouse, the `csi-yadro` module is used. It allows the creation of StorageClass resources through custom resources like [YadroStorageClass](../../../reference/cr/yadrostorageclass/). To enable the module, run the following command:

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

Wait until `csi-yadro` is in the `Ready` status. To check the status, run the following command:

```shell
d8 k get module csi-yadro -w
```

In the output, you should see information about the module:

```console
NAME        WEIGHT   STATE     SOURCE     STAGE   STATUS
csi-yadro   910      Enabled   Embedded           Ready
```

## Connect to the TATLIN.UNIFIED storage system

To connect to the `TATLIN.UNIFIED` storage system and enable configuring of StorageClass objects, apply the following [YadroStorageConnection](../../../reference/cr/yadrostorageconnection/) resource:

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

To create a StorageClass, use the [YadroStorageClass](../../../reference/cr/yadrostorageclass/) resource. Creating a StorageClass resource manually without using [YadroStorageClass](../../../reference/cr/yadrostorageclass/) can lead to errors.

Example command to create a StorageClass based on the `TATLIN.UNIFIED` storage system:

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

To make sure the `csi-yadro` is working properly, check the pod status in the `d8-csi-yadro` namespace. All pods must have the `Running` or `Completed` status. The `csi-yadro` pods must be running on all nodes.

To check that the module works, run the following command:

```shell
d8 k -n d8-csi-yadro get pod -owide -w
```
