---
title: "HPE data storage"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/external/hpe.html
---

Deckhouse Virtualization Platform (DVP) includes support for HPE 3PAR storage systems, enabling volume management in Kubernetes using a CSI driver. This integration provides reliable, scalable, and high-performance storage suitable for mission-critical workloads. The `csi-hpe` module is used to work with HPE 3PAR systems, allowing StorageClass creation in Kubernetes through the [HPEStorageClass](/modules/csi-hpe/stable/cr.html#hpestorageclass) resource.

{% alert level="warning" %}
User-created StorageClass for the `csi.hpe.com` CSI driver is not allowed.  
Only HPE 3PAR storage systems are supported. For other HPE storage systems, contact the [technical support](https://deckhouse.io/tech-support/).
{% endalert %}

This page provides instructions on connecting HPE 3PAR to DVP, configuring the connection, creating StorageClass, and verifying storage functionality.

## System requirements

- A deployed and configured HPE storage system.
- Unique IQNs in `/etc/iscsi/initiatorname.iscsi` on each Kubernetes node.

## Configuration

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling the module

Enable the `csi-hpe` module. This will result in the following actions across all cluster nodes:

- Registration of the CSI driver.
- Launch of service pods for the `csi-hpe` components.

```shell
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

Wait until the module transitions to the `Ready` state. Check the module’s status with the following command:

```shell
d8 k get module csi-hpe -w
```

### Creating a StorageClass

To create a StorageClass, you need to use the [HPEStorageClass](/modules/csi-hpe/stable/cr.html#hpestorageclass) and [HPEStorageConnection](/modules/csi-hpe/stable/cr.html#hpestorageconnection) resource. Here is an example command to create such a resource:

- Creating a HPEStorageConnection resource:

  ```shell
  d8 k apply -f -<<EOF
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: HPEStorageConnection
  metadata:
    name: hpe
  spec:
    controlPlane:
      backendAddress: "172.17.1.55" # Storage system address (mutable).
      username: "3paradm" # API username (mutable).
      password: "3pardata" # API password (mutable).
      serviceName: "primera3par-csp-svc"
      servicePort: "8080"
  EOF
  ```

  Verify the creation of the object using the following command (`Phase` should be `Created`):

  ```shell
  d8 k get hpestorageconnections.storage.deckhouse.io <hpestorageconnection-name>
  ```

- Creating a HPEStorageClass resource:

  ```shell
  d8 k apply -f -<<EOF
  apiVersion: storage.deckhouse.io/v1alpha1
  kind: HPEStorageClass
  metadata:
    name: hpe
  spec:
    pool: "test-cpg"
    accessProtocol: "iscsi" # fc or iscsi (iscsi by default), immutable.
    fsType: "xfs" # xfs, ext3, ext4, btrfs (ext4 by default), mutable.
    storageConnectionName: "hpe" # Immutable.
    reclaimPolicy: Delete # Delete or Retain.
    cpg: "test-cpg"
  EOF
  ```

  Verify the creation of the object using the following command (`Phase` should be `Created`):

  ```shell
  d8 k get hpestorageclasses.storage.deckhouse.io <hpestorageclass-name>
  ```

### Module health verification

To verify module health, ensure that all pods in the `d8-csi-hpe` namespace are in the `Running` or `Completed` state and are running on every node in the cluster:

```shell
d8 k -n d8-csi-hpe get pod -owide -w
```
