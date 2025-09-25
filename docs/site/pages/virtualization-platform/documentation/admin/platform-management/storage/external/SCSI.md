---
title: "SCSI-based data storage"
permalink: en/virtualization-platform/documentation/admin/platform-management/storage/external/scsi.html
---

Deckhouse Virtualization Platform (DVP) supports managing storage connected via iSCSI or Fibre Channel, enabling working with volumes at the block device level. This allows for the integration of storage systems with Kubernetes and management through a CSI driver.

This page provides instructions for connecting SCSI devices in DVP, creating SCSITarget, StorageClass, and verifying system functionality.

## Supported features

- Detecting LUN via iSCSI/FC.
- Creating PersistentVolume from pre-provisioned LUN.
- Deleting PersistentVolume and wiping data on LUN.
- Attaching LUN to nodes via iSCSI/FC.
- Creating `multipath` devices and mounting them in VMs.
- Detaching LUN from nodes.

## Limitations

- Creating LUN on storage is not supported.
- Resizing LUN is not possible.
- Snapshots are not supported.

## System requirements

- A deployed and configured storage system with SCSI connections.
- Unique IQN values in `/etc/iscsi/initiatorname.iscsi` on each Kubernetes node.

## Quick start

All commands should be executed on a machine with access to the Kubernetes API and administrator rights.

### Enabling the module

Enable the `csi-scsi-generic` module. This will ensure that the following happens on all cluster nodes:

- The CSI driver is registered.
- Auxiliary pods for the `csi-scsi-generic` components are launched.

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

Wait for the module to transition to the `Ready` state.

```shell
d8 k get module csi-scsi-generic -w
```

### Creating an SCSITarget

To create an SCSITarget, use the [SCSITarget](/modules/csi-scsi-generic/stable/cr.html#scsitarget). An example of commands to create such a resource:

```shell
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

Note that the example above uses two SCSITargets. You can create multiple SCSITargets for either the same or different storage systems. This allows for the use of multipath to improve failover and performance.

To verify that the object has been created (`Phase` should be `Created`), run:

```shell
d8 k get scsitargets.storage.deckhouse.io <name-of-scsitarget>
```

### Creating a StorageClass

To create a StorageClass, use the [SCSIStorageClass](/modules/csi-scsi-generic/stable/cr.html#scsistorageclass). An example of commands to create such a resource:

```shell
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

Pay attention to the `scsiDeviceSelector`. This field is used to select the SCSITarget for PV creation based on labels. In the example above, all SCSITargets with the label `my-key: some-label-value` are selected. This label will be applied to all devices detected within the specified SCSITarget.

To verify that the object has been created (`Phase` should be `Created`), run:

```shell
d8 k get scsistorageclasses.storage.deckhouse.io <name-of-scsistorageclass>
```

### Module health check

Check the status of pods in the `d8-csi-scsi-generic` namespace using the following command. All pods should be in the `Running` or `Completed` state and deployed on all nodes.

```shell
d8 k -n d8-csi-scsi-generic get pod -owide -w
```
