---
title: "HPE data storage"
permalink: en/admin/storage/external/hpe.html
---

Deckhouse provides support for HPE 3PAR storage systems, enabling volume management in Kubernetes using a CSI driver. This ensures a reliable, scalable, and high-performance storage solution suitable for mission-critical workloads. To support HPE 3PAR storage systems, the `csi-hpe` module is used, allowing the creation of StorageClass in Kubernetes through the [HPEStorageClass](../../../reference/cr/hpestorageclass/) resource.

{% alert level="warning" %}
User-created StorageClass for the `csi.hpe.com` CSI driver is not allowed.  
Only HPE 3PAR** storage systems are supported. For other HPE storage systems, please contact technical support.
{% endalert %}

This page provides instructions on connecting HPE 3PAR to Deckhouse, configuring the connection, creating StorageClass, and verifying storage functionality.

## System requirements

- A deployed and configured HPE storage system.
- Unique IQNs in `/etc/iscsi/initiatorname.iscsi` on each Kubernetes node.

## Setup and Configuration

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling the module

Enable the `csi-hpe` module. This will result in the following actions across all cluster nodes:
- registration of the CSI driver.
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

Wait for the module to become `Ready`.

```shell
kubectl get module csi-hpe -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [HPEStorageClass](../../../reference/cr/hpestorageclass/) and [HPEStorageConnection](../../../reference/cr/hpestorageconnection/) resource. Here is an example command to create such a resource:

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

You can check objects creation (Phase must be `Created`):

```shell
d8 k get hpestorageconnections.storage.deckhouse.io <hpestorageconnection name>
```

```shell
d8 k get hpestorageclasses.storage.deckhouse.io <hpestorageclass name>
```

### Module Health Verification

To verify module health, ensure that all pods in the `d8-csi-hpe` namespace are in the `Running` or `Completed` state and are running on every node in the cluster:

```shell
d8 k -n d8-csi-hpe get pod -owide -w
```
