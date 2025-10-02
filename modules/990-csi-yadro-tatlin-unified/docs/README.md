---
title: "The csi-yadro-tatlin-unified module"
description: "The csi-yadro-tatlin-unified module: General Concepts and Principles."
d8Edition: ee
---

The module installs and configures the CSI driver for SAN TATLIN.UNIFIED. The module allows you to create a `StorageClass` in `Kubernetes` by creating [Kubernetes custom resources](./cr.html#yadrotatlinunifiedstorageclass) `YadroTatlinUnifiedStorageClass`.

> **Caution!** The user is not allowed to create a `StorageClass` for the `csi-tatlinunified.yadro.com` CSI driver.

{% alert level="info" %}
For working with snapshots, the [snapshot-controller](../../snapshot-controller/) module must be connected.
{% endalert %}

## System requirements and recommendations

### Requirements

- Presence of a deployed and configured TATLIN.UNIFIED SAN.
- Unique iqn in /etc/iscsi/initiatorname.iscsi on each of Kubernetes Nodes

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling module

- Enable the `csi-yadro-tatlin-unified` module. This will result in the following actions across all cluster nodes:
    - registration of the CSI driver;
    - launch of service pods for the `csi-yadro-tatlin-unified` components.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-yadro-tatlin-unified
spec:
  enabled: true
  version: 1
EOF
```

- Wait for the module to become `Ready`.

```shell
kubectl get module csi-yadro-tatlin-unified -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [YadroTatlinUnifiedStorageClass](./cr.html#yadrotatlinunifiedstorageclass) and [YadroTatlinUnifiedStorageConnection](./cr.html#yadrotatlinunifiedstorageconnection) resource. Here is an example command to create such a resource:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: YadroTatlinUnifiedStorageConnection
metadata:
  name: yad1
spec:
  controlPlane:
    address: "172.19.28.184"
    username: "admin"
    password: "cGFzc3dvcmQ=" # MUST BE BASE64 ENCODED
    ca: "base64encoded"
    skipCertificateValidation: true
  dataPlane:
    protocol: "iscsi"
    iscsi:
      volumeExportPort: "p50,p51,p60,p61"
EOF
```

```yaml
kubectl apply -f -<<EOF
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

- You can check objects creation (Phase must be `Created`):

```shell
kubectl get yadrotatlinunifiedstorageconnections.storage.deckhouse.io <yadrotatlinunifiedstorageconnection name>
```

```shell
kubectl get yadrotatlinunifiedstorageclasses.storage.deckhouse.io <yadrotatlinunifiedstorageclass name>
```

### Checking module health

You can verify the functionality of the module using the instructions [here](./faq.html#how-to-check-module-health)
