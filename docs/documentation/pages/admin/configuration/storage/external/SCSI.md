---
title: "SCSI-based data storage"
permalink: en/admin/configuration/storage/external/scsi.html
---

Deckhouse Kubernetes Platform (DKP) supports managing storage connected via iSCSI or Fibre Channel, enabling working with volumes at the block device level. This allows for the integration of storage systems with Kubernetes and management through a CSI driver.

This page provides instructions for connecting SCSI devices in DKP, creating SCSITarget, StorageClass, and verifying system functionality.

## Supported features

DKP supports:

- Detecting Logical Unit Numbers (LUN) via iSCSI or Fibre Channel.
- Creating PersistentVolume (PV) from pre-provisioned LUN.
- Deleting PersistentVolume and wiping data on LUN.
- Attaching LUN to nodes via iSCSI or Fibre Channel.
- Creating multipath devices and mounting them in pods.
- Detaching LUN from nodes.

## Limitations

DKP doesn't support:

- Creating LUN on storage systems.
- Resizing LUN.
- Creating snapshots.

## System requirements

The following requirements are applicable to the infrastructure and cluster nodes:

- A deployed and configured storage system providing access to LUN via iSCSI or Fibre Channel.

- For iSCSI connections:
  - A unique iSCSI Qualified Name (IQN) must be configured on each cluster node in the `/etc/iscsi/initiatorname.iscsi` file.
  - The `multipath-tools` package must be installed on nodes.

- For Fibre Channel connections:
  - Fibre Channel Host Bus Adapters (FC HBA) must be installed and available on cluster nodes (`/sys/class/fc_host/host*`).
  - In the Storage Area Network (SAN), LUN zoning and masking must be configured, providing node initiators with access to the storage target ports.
  - Necessary LUNs must be pre-provisioned on the storage system. DKP does not create LUNs.
  - The `multipath-tools` package must be installed on nodes.

## Quick start

All commands should be executed on a machine with access to the Kubernetes API and administrator rights.

### Enabling the csi-scsi-generic module

Enable the [`csi-scsi-generic`](/modules/csi-scsi-generic/) module using the command below. This will ensure that the following happens on all cluster nodes:

- The CSI driver is registered.
- Auxiliary pods for the `csi-scsi-generic` components are running.

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

The [SCSITarget](/modules/csi-scsi-generic/cr.html#scsitarget) resource describes connection to a single SCSI target. When creating a resource in `spec`, specify one of the connection types: [`iSCSI`](/modules/csi-scsi-generic/cr.html#scsitarget-v1alpha1-spec-iscsi) or [`fibreChannel`](/modules/csi-scsi-generic/cr.html#scsitarget-v1alpha1-spec-fibrechannel).

#### iSCSI

The following is a configuration example for connecting via iSCSI.
In this example, two [SCSITarget](/modules/csi-scsi-generic/cr.html#scsitarget) resources are created.

You can create multiple resources for either the same or different storage systems. This lets you use multipath to improve failover and performance.

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

To verify that the object has been created, run:

```shell
d8 k get scsitargets.storage.deckhouse.io <name-of-scsitarget>
```

An object is considered to be created successfully if it says `Created` in the corresponding `Phase` column in the output.

#### Fibre Channel

To configure a connection based on Fibre Channel, do the following:

1. Before creating the SCSITarget resource, configure zoning and LUN masking on the SAN so that cluster nodes can access the necessary LUNs.

1. Create the SCSITarget resource. In the [`spec.fibreChannel.WWNs`](/modules/csi-scsi-generic/cr.html#scsitarget-v1alpha1-spec-fibrechannel-wwns) field, define a World Wide Port Name (WWPN) of the target storage ports, which expose the necessary LUNs. DKP discovers devices by matching the defined WWPNs against paths under `/dev/disk/by-path/`.

   Define the WWPNs in one of the following formats:

   - 16 hexadecimal characters (`2001c89f1acd6117`)
   - With colons (`20:01:c8:9f:1a:cd:61:17`)
   - With the `0x` prefix (`0x2001c89f1acd6117`)

   DKP triggers an FC host scan during discovery and volume attach. It does not configure the commutators or log in to targets.

   Configuration example with two target storage system ports for multipath:

   ```shell
   kubectl apply -f -<<EOF
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: SCSITarget
   metadata:
     name: hpe-3par-fc-1
   spec:
     deviceTemplate:
       metadata:
         labels:
           my-key: some-label-value
     fibreChannel:
       WWNs:
       - 2001c89f1acd6117

   ---
   apiVersion: storage.deckhouse.io/v1alpha1
   kind: SCSITarget
   metadata:
     name: hpe-3par-fc-2
   spec:
     deviceTemplate:
       metadata:
         labels:
           my-key: some-label-value
     fibreChannel:
       WWNs:
       - 2001c89f1acd6118
   EOF
   ```

1. Before applying the resource verify that FC HBA are available on the node and that LUN paths are visible after SAN zoning:

   ```shell
   # FC hosts present.
   ls /sys/class/fc_host/
   
   # Target WWPNs and LUNs available after zoning.
   ls -l /dev/disk/by-path/ | grep -E 'fc-|/fc-'
   ```

   Possible results:

   - If the `/sys/class/fc_host/` directory is empty, the node has no FC HBA or the corresponding driver is not loaded.
   - If there are no `fc-*` entries in the `/dev/disk/by-path/` directory, check zoning and LUN masking settings on the storage system.
   - If the FC paths are displayed, the node is ready to detect LUN provided via Fibre Channel.

1. After the SCSITarget is created, the controller discovers available LUNs and creates SCSIDevice objects. Use the same [SCSIStorageClass](/modules/csi-scsi-generic/cr.html#scsistorageclass) resource and [`scsiDeviceSelector`](/modules/csi-scsi-generic/cr.html#scsistorageclass-v1alpha1-spec-scsideviceselector) workflow as for iSCSI.

1. To verify that the object has been created, run the following command:

   ```shell
   d8 k get scsitargets.storage.deckhouse.io <name-of-scsitarget>
   ```

   An object is considered to be created successfully if it says `Created` in the corresponding `Phase` column in the output.

### Creating a StorageClass

To create a StorageClass, use the [SCSIStorageClass](/modules/csi-scsi-generic/cr.html#scsistorageclass) resource. A command example to create such a resource:

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

The [`scsiDeviceSelector`](/modules/csi-scsi-generic/cr.html#scsistorageclass-v1alpha1-spec-scsideviceselector) field is used to select the SCSITarget for PV creation based on labels. In the example above, all SCSITargets with the label `my-key: some-label-value` are selected. This label will be applied to all devices detected within the specified SCSITarget.

To verify that the object has been created, run:

```shell
d8 k get scsistorageclasses.storage.deckhouse.io <name-of-scsistorageclass>
```

An object is considered to be created successfully if it says `Created` in the corresponding `Phase` column in the output.

### Csi-scsi-generic module health check

Check the status of pods in the `d8-csi-scsi-generic` namespace using the command below. All pods should be in the `Running` or `Completed` state and deployed on all nodes.

```shell
d8 k -n d8-csi-scsi-generic get pod -owide -w
```
