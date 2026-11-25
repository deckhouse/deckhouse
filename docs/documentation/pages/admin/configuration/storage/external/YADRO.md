---
title: "TATLIN.UNIFIED (Yadro) unified storage"
permalink: en/admin/configuration/storage/external/yadro.html
d8Revision: ee
---

Deckhouse supports integration with the TATLIN.UNIFIED (Yadro) storage system, enabling volume management in Kubernetes. This allows the use of centralized storage for containerized workloads, ensuring high performance and fault tolerance.

This page provides instructions on connecting TATLIN.UNIFIED (Yadro) to Deckhouse, configuring the connection, creating a StorageClass, and verifying system functionality.

## Enabling the module

To manage volumes based on the TATLIN.UNIFIED (Yadro) storage system in Deckhouse, the [`csi-yadro-tatlin-unified`](/modules/csi-yadro-tatlin-unified/) module is used. It allows the creation of StorageClass resources through custom resources like [YadroTatlinUnifiedStorageClass](/modules/csi-yadro-tatlin-unified/cr.html#yadrotatlinunifiedstorageclass). To enable the module, run the following command:

```shell
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

<!-- markdownlint-disable MD031 -->
```console
NAME                       STAGE   SOURCE    PHASE       ENABLED    READY
si-yadro-tatlin-unified            Embedded  Available   True       True
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

## Connecting to the TATLIN.UNIFIED storage system

To connect to the `TATLIN.UNIFIED` storage system and enable configuring of StorageClass objects, apply the following [YadroTatlinUnifiedStorageConnection](/modules/csi-yadro-tatlin-unified/cr.html#yadrotatlinunifiedstorageconnection) resource:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageConnection
metadata:
  name: yad1
spec:
  controlPlane:
    address: "172.19.28.184"
    username: "admin"
    password: "cGFzc3dvcmQ=" # A Base64‑encoded password. Encoding is performed by the command: echo -n '<PASSWORD>' | base64 -w0
    ca: "base64encoded"
    skipCertificateValidation: true
  dataPlane:
    protocol: "iscsi"
    iscsi:
      volumeExportPort: "p50,p51,p60,p61"
EOF
```

The `spec.controlPlane.password` field contains the Base64-encoded password in the standard Base64 alphabet with the `=` appended and no trailing newline. To encode, use the following command:

```shell
echo -n '<password>' | base64 -w0
```

`echo` without `-n` appends a newline to the password, and `-w0` prevents GNU `base64` from splitting a long value across multiple lines. The API server rejects a value that is not Base64. A Base64 value that decodes to a password with a space or newline at the beginning or end, or to bytes that are not UTF-8 text, is accepted, and the connection goes to the `Ready` state of `False` with reason `InvalidPassword`. To see the reason, use the command `d8 k get yadrotatlinunifiedstorageconnection -o wide`.

## Creating a StorageClass

To create a StorageClass, use the [YadroTatlinUnifiedStorageClass](/modules/csi-yadro-tatlin-unified/cr.html#yadrotatlinunifiedstorageclass) resource. Creating a StorageClass resource manually without using [YadroTatlinUnifiedStorageClass](/modules/csi-yadro-tatlin-unified/cr.html#yadrotatlinunifiedstorageclass) can lead to errors.

Example command to create a StorageClass based on the `TATLIN.UNIFIED` storage system:

```shell
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

## Ensuring the module works

To make sure the [`csi-yadro-tatlin-unified`](/modules/csi-yadro-tatlin-unified/) module is working properly, check the pod status in the `d8-csi-yadro-tatlin-unified` namespace. All pods must have the `Running` or `Completed` status. The `csi-yadro-tatlin-unified` pods must be running on all nodes.

To check that the module works, run the following command:

```shell
d8 k -n d8-csi-yadro-tatlin-unified get pod -owide -w
```
