---
title: "The csi-scsi-generic module"
description: "CSI SCSI GENERIC module: general concepts"
d8Edition: ee
moduleStatus: experimental
---

The module provides a CSI for managing volumes using storage systems connected via SCSI.

Currently supported features:
- LUN discovery via iSCSI
- Creation of PV from pre-provisioned LUNs
- Deletion of PV and wiping data on the LUN
- Connecting LUN to nodes via iSCSI
- Creating multipath devices and mounting them to pods
- Detaching LUN from nodes

Not supported:
- LUN creation on the storage system
- Resizing LUN
- Creating snapshots

## System Requirements and Recommendations

### Requirements

- A deployed and configured storage system with SCSI connections.
- Unique iqn values in /etc/iscsi/initiatorname.iscsi on each Kubernetes Node.

## Quick Start

All commands should be executed on a machine with access to the Kubernetes API and administrator rights.

### Enabling the Module

- Enable the `csi-scsi-generic` module. This will ensure that the following happens on all cluster nodes:
    - The CSI driver is registered.
    - Auxiliary pods for the `csi-scsi-generic` components are launched.

```shell
kubectl apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-scsi-generic
spec:
  enabled: true
  version: 1
EOF
```

- Wait for the module to transition to the `Ready` state.

```shell
kubectl get module csi-scsi-generic -w
```

### Creating an SCSITarget

To create an SCSITarget, use the [SCSITarget](./cr.html#scsitarget). An example of commands to create such a resource:

```yaml
kubectl apply -f -<<EOF
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

<!-- - To verify that the object has been created (Phase should be `Created`), run:

```shell
kubectl get scsitargets.storage.deckhouse.io <имя scsitarget>
``` -->

### Creating a StorageClass

To create a StorageClass, use the [SCSIStorageClass](./cr.html#scsistorageclass). An example of commands to create such a resource:

```yaml
kubectl apply -f -<<EOF
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

- To verify that the object has been created (Phase should be `Created`), run:

```shell
kubectl get scsistorageclasses.storage.deckhouse.io <имя scsistorageclass>
```

### PV cleanup

Because of we do not manage **iSCSI Target** server and re-use  available volumes from **iSCSI Target**,  we must cleanup them after removing **PV**. 
**PV** cleanup mode change is automatic and depends from **trim** support.
Clean-up process step by step:
1. Checking **trim** support. **Trim** support checks by read "discard_max_bytes" from  `/sys/block/${device name}/queue/discard_max_bytes`, if size > 0, **trim** supported
2. Clean-up:
    1. If **trim** supported, command be like `blkdiscard ${device name}`
    2. If **trim** not supported, commend be like `blkdiscard -z ${device name}`, `-z` mean what device will be zeroed(fully writed by zeroes)
  
