---
title: "TATLIN.UNIFIED (Yadro) unified storage"
permalink: en/storage/admin/external/yadro.html
d8Revision: ee
---

{% alert level="info" %}
Available in some commercial editions:  **EE**
{% endalert %}

Deckhouse supports integration with the [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) storage system, enabling volume management in Kubernetes. This allows the use of centralized storage for containerized workloads, ensuring high performance and fault tolerance.

This page provides instructions on connecting [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) to Deckhouse, configuring the connection, creating a StorageClass, and verifying system functionality.

## Enabling the module

To manage volumes based on the [TATLIN.UNIFIED (Yadro)](https://yadro.com/ru/tatlin/unified) storage system in Deckhouse, the `csi-yadro-tatlin-unified` module is used. It allows the creation of StorageClass resources through custom resources like [YadroTatlinUnifiedStorageClass](../../../reference/cr/yadrotatlinunifiedstorageclass/). To enable the module, run the following command:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-yadro-tatlin-unified
spec:
  enabled: true
  version: 1
EOF
```

Wait until `csi-yadro-tatlin-unified` is in the `Ready` status. To check the status, run the following command:

```shell
d8 k get module csi-yadro-tatlin-unified -w
```

In the output, you should see information about the module:

```console
NAME                       STAGE   SOURCE   PHASE       ENABLED   READY
csi-yadro-tatlin-unified                    Available   True      True
```

## Connect to the TATLIN.UNIFIED storage system

To connect to the `TATLIN.UNIFIED` storage system and enable configuring of StorageClass objects, apply the following [YadroTatlinUnifiedStorageConnection](../../../reference/cr/yadrotatlinunifiedstorageconnection/) resource:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageConnection
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

To create a StorageClass, use the [YadroTatlinUnifiedStorageClass](../../../reference/cr/yadrotatlinunifiedstorageclass/) resource. Creating a StorageClass resource manually without using [YadroTatlinUnifiedStorageClass](../../../reference/cr/yadrotatlinunifiedstorageclass/) can lead to errors.

Example command to create a StorageClass based on the `TATLIN.UNIFIED` storage system:

```yaml
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageClass
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

To make sure the `csi-yadro-tatlin-unified` is working properly, check the pod status in the `d8-csi-yadro-tatlin-unified` namespace. All pods must have the `Running` or `Completed` status. The `csi-yadro-tatlin-unified` pods must be running on all nodes.

To check that the module works, run the following command:

```shell
d8 k -n d8-csi-yadro-tatlin-unified get pod -owide -w
```
