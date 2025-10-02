---
title: "The csi-netapp module"
description: "The csi-netapp module: General Concepts and Principles."
d8Edition: ee
---

The module installs and configures the CSI driver for Netapp SAN. The module allows you to create a `StorageClass` in `Kubernetes` by creating [Kubernetes custom resources](./cr.html#yadrostorageclass) `YadroStorageClass`.

> **Caution!** The user is not allowed to create a `StorageClass` for the `csi.Netapp.com` CSI driver.

> **Caution!** At the moment, the module supports SAN that are compatible with [NetApp's Trident CSI](https://github.com/NetApp/trident). For other Netapp SAN support please contact tech support.

{% alert level="info" %}
For working with snapshots, the [snapshot-controller](../../snapshot-controller/) module must be connected.
{% endalert %}

## System requirements and recommendations

### Requirements

- Presence of a deployed and configured Netapp SAN.
- Unique iqn in /etc/iscsi/initiatorname.iscsi on each of Kubernetes Nodes

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling module

- Enable the `csi-netapp` module. This will result in the following actions across all cluster nodes:
  - registration of the CSI driver;
  - launch of service pods for the `csi-netapp` components.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-netapp
spec:
  enabled: true
  version: 1
EOF
```

- Wait for the module to become `Ready`.

```shell
kubectl get module csi-netapp -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [NetappStorageClass](./cr.html#Netappstorageclass) and [NetappStorageConnection](./cr.html#Netappstorageconnection) resource. Here is an example command to create such a resource:

```yaml
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NetappStorageConnection
metadata:
  name: Netapp
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
kubectl apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NetappStorageClass
metadata:
  name: Netapp
spec:
  pool: "test-cpg"
  accessProtocol: "iscsi" # fc или iscsi (default iscsi), immutable
  fsType: "xfs" # xfs, ext3, ext4 (default ext4), mutable
  storageConnectionName: "Netapp" # immutable
  reclaimPolicy: Delete # Delete of Retain
  cpg: "test-cpg"
EOF
```

- You can check objects creation (Phase must be `Created`):

```shell
kubectl get Netappstorageconnections.storage.deckhouse.io <Netappstorageconnection name>
```

```shell
kubectl get Netappstorageclasses.storage.deckhouse.io <Netappstorageclass name>
```

### Checking module health

You can verify the functionality of the module using the instructions [here](./faq.html#how-to-check-module-health)
