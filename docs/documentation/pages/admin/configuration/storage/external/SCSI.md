---
title: "SCSI-based data storage"
permalink: en/admin/configuration/storage/external/scsi.html
---

Deckhouse supports managing storage connected via iSCSI or Fibre Channel, enabling working with volumes at the block device level. This allows for the integration of storage systems with Kubernetes and management through a CSI driver.

This page provides instructions for connecting SCSI devices in Deckhouse, creating SCSITarget, StorageClass, and verifying system functionality.

### Supported Features

- Detecting LUN via iSCSI/FC.
- Creating PersistentVolume from pre-provisioned LUN.
- Deleting PersistentVolume and wiping data on LUN.
- Attaching LUN to nodes via iSCSI/FC.
- Creating `multipath` devices and mounting them in pods.
- Detaching LUN from nodes.

### Limitations

- Creating LUN on storage is not supported.
- Resizing LUN is not possible.
- Snapshots are not supported.

## System Requirements

- A properly configured and available storage system with iSCSI/FC connectivity.
- Unique IQN assigned to each Kubernetes node in `/etc/iscsi/initiatorname.iscsi`.

## Setup and Configuration

All commands should be executed on a machine with administrative access to the Kubernetes API.

### Enabling the module

To work with storage connected via SCSI, enable the `csi-scsi-generic` module. This will result in:
- CSI driver registration.
- The launch of `csi-scsi-generic` service pods.

```shell
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-scsi-generic
spec:
  enabled: true
  version: 1
EOF
```

Wait for the module to transition to the `Ready` state. Verify the module’s status using the command:

```shell
d8 k get module csi-scsi-generic -w
```

### Creating an SCSITarget

To work with SCSI devices, [SCSITarget](../../../reference/cr/scsitarget/) resources must be created.

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSITarget
metadata:
  name: hpe-3par-1
spec:
  deviceTemplate:
    metadata:
      labels:
        my-key: some-label-value
  iSCSI:
    auth:
      login: ""
      password: ""
    iqn: iqn.2000-05.com.3pardata:xxxx1
    portals:
    - 192.168.1.1

---
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSITarget
metadata:
  name: hpe-3par-2
spec:
  deviceTemplate:
    metadata:
      labels:
        my-key: some-label-value
  iSCSI:
    auth:
      login: ""
      password: ""
    iqn: iqn.2000-05.com.3pardata:xxxx2
    portals:
    - 192.168.1.2
EOF

```

An example of commands to create a resource with FC connection:

```shell
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSITarget
metadata:
  name: scsi-target-2
spec:
  fibreChannel:
    WWNs:
      - 00:00:00:00:00:00:00:00
      - 00:00:00:00:00:00:00:01
  deviceTemplate:
    metadata:
      labels:
        some-label-key: some-label-value1
EOF
```

Note that the example above uses two [SCSITargets](../../../reference/cr/scsitarget/). You can create multiple [SCSITargets](../../../reference/cr/scsitarget/) for either the same or different storage systems. This allows for the use of `multipath` to improve failover and performance.

Verify the creation of the object with the following command. The `Phase` field should be `Created`:

```shell
d8 k get scsitargets.storage.deckhouse.io <scsitarget name>
```

### Creating a StorageClass

To create a StorageClass, use the [SCSIStorageClass](../../../reference/cr/scsistorageclass/) resource. An example of commands to create such a resource:

```yaml
d8 k apply -f -<<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: SCSIStorageClass
metadata:
  name: scsi-all
spec:
  scsiDeviceSelector:
    matchLabels:
      my-key: some-label-value
  reclaimPolicy: Delete
EOF
```

Pay attention to the `scsiDeviceSelector`. This field is used to select the [SCSITarget](../../../reference/cr/scsitarget/) for PV creation based on labels. In the example above, all [SCSITargets](../../../reference/cr/scsitarget/) with the label `my-key: some-label-value` are selected. This label will be applied to all devices detected within the specified [SCSITarget](../../../reference/cr/scsitarget/).

Verify the creation of the object with the following command. The `Phase` field should be `Created`.

```shell
d8 k get scsistorageclasses.storage.deckhouse.io <scsistorageclass name>
```

### Module Health Check

Check the status of pods in the `d8-csi-scsi-generic` namespace using the following command. All pods should be in the `Running` or `Completed` state and deployed on all nodes.

```shell
d8 k -n d8-csi-scsi-generic get pod -owide -w
```
