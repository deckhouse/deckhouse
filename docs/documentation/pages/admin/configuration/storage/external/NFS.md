---
title: "NFS storage"
permalink: en/admin/configuration/storage/external/nfs.html
description: "Configure NFS storage integration in Deckhouse Kubernetes Platform. CSI driver setup, StorageClass configuration, RPC-with-TLS security, and NFS server connection guide."
---

Deckhouse Kubernetes Platform (DKP) supports integration with Network File System (NFS), providing the ability to use network file storage as Kubernetes volumes. The [`csi-nfs`](/modules/csi-nfs/) module provides a CSI driver for connecting to NFS servers and creating PersistentVolumes based on them.

This page provides instructions for configuring NFS storage in DKP, including connecting to an NFS server, creating StorageClass, configuring RPC-with-TLS security, and verifying system functionality.

## System Requirements

### Minimum Requirements

The following conditions must be met for the [`csi-nfs`](/modules/csi-nfs/) module to work:

- [Supported Linux distributions](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#linux) with appropriate kernels.
- Configured and accessible NFS server.
- For RPC-with-TLS support: Linux kernel with enabled `CONFIG_TLS` and `CONFIG_NET_HANDSHAKE` options.

### Configuration Recommendations

For optimal module operation, it is recommended to:

- For automatic pod restarts when TLS parameters change, enable the [`pod-reloader`](/modules/pod-reloader/) module (enabled by default).
- Use stable versions of NFS servers with support for required protocols.

{% alert level="warning" %}
For NFS to work as virtual disk storage in Deckhouse Virtualization Platform, configure the NFS server with the `no_root_squash` option.
{% endalert %}

## Limitations

### General Limitations

The following limitations apply when working with the [`csi-nfs`](/modules/csi-nfs/) module:

- Manual creation of StorageClass for CSI driver `nfs.csi.k8s.io` is prohibited — use the [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass) resource.
- PersistentVolumes are created only through [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass) resources.
- Changing NFS server parameters in already created PVs is impossible.

### Volume Snapshot Limitations

{% alert level="info" %}
A connected [`snapshot-controller`](/modules/snapshot-controller/) module is required for working with snapshots.
{% endalert %}

Creating NFS volume snapshots has significant limitations related to the NFS architecture and the way they are implemented in [`csi-nfs`](/modules/csi-nfs/). Avoid using snapshots in module whenever possible.

**Snapshot Operation Principle:**
- The CSI driver creates a snapshot at the NFS server level.
- The `tar` utility is used for archiving, which imposes certain limitations.
- The archive is saved in the root folder of the NFS server specified in the [`spec.connection.share`](/modules/csi-nfs/cr.html#nfsstorageclass-v1alpha1-spec-connection-share) parameter.

{% alert level="warning" %}
- Before creating a snapshot, **mandatory** stop the workload (pods) using the NFS volume.
- NFS does not ensure atomicity of operations at the file system level when creating snapshots.
{% endalert %}

### RPC-with-TLS Mode Limitations

#### Functional Limitations

The following limitations apply when using RPC-with-TLS:

- Only one client certificate is supported for the `mtls` security policy.
- One NFS server cannot simultaneously work in different security modes: `tls`, `mtls` and standard mode (without TLS).
- The `tlshd` daemon should not be running on cluster nodes, otherwise it will conflict with the module daemon. To prevent conflicts when enabling TLS on nodes, third-party `tlshd` is automatically stopped and its autostart is disabled.

#### System Requirements

The following system requirements must be met for RPC-with-TLS operation:

- Linux kernel must be compiled with enabled `CONFIG_TLS` and `CONFIG_NET_HANDSHAKE` parameters.
- `nfs-utils` package (in Debian-based distributions — `nfs-common`) version >= 2.6.3.

## Configuration

{% alert level="info" %}
All commands must be executed on a machine with administrative rights in the Kubernetes API.
{% endalert %}

The following steps are required to configure NFS storage:

- Module enabling.
- Creating [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass).

### Enabling the Module

To support working with NFS storage, enable the [`csi-nfs`](/modules/csi-nfs/) module, which allows creating StorageClass in Kubernetes using custom [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass) resources. After enabling the module, the following will happen on cluster nodes:

- CSI driver registration.
- Launch of `csi-nfs` service pods and creation of necessary components.

```shell
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: csi-nfs
spec:
  enabled: true
  version: 1
EOF
```

Wait for the module to transition to the `Ready` state:

```shell
d8 k get module csi-nfs -w
```

Check the status of pods in the `d8-csi-nfs` namespace. All pods should be in `Running` or `Completed` state and running on all nodes:

```shell
d8 k -n d8-csi-nfs get pod -owide -w
```

### Creating StorageClass

To create a StorageClass, you must use the [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass) resource. Example of creating a resource:

```shell
d8 k apply -f - <<EOF
apiVersion: storage.deckhouse.io/v1alpha1
kind: NFSStorageClass
metadata:
  name: nfs-storage-class
spec:
  connection:
    host: 10.223.187.3
    share: /
    nfsVersion: "4.1"
  reclaimPolicy: Delete
  volumeBindingMode: WaitForFirstConsumer
EOF
```

For each PV, a directory `<share directory>/<PV name>` will be created.

### Volume Cleanup Configuration

When deleting a PersistentVolume, files with user data may remain on the volume. To ensure data security, you can configure the volume cleanup method before deletion using the [`volumeCleanup`](/modules/csi-nfs/cr.html#nfsstorageclass-v1alpha1-spec-volumecleanup) parameter.

{% alert level="warning" %}
Important notes on volume cleanup:

- The cleanup option does not affect files already deleted by the client application.
- Cleanup is performed only through the NFS protocol and depends on:
  - NFS server service.
  - File system.
  - Block device level and their virtualization (e.g., LVM).
  - Physical devices.
- Ensure NFS server trustworthiness before sending sensitive data.
{% endalert %}

#### Parameter `RandomFillSinglePass`

File contents are overwritten with a random sequence before deletion. The random sequence is transmitted over the network.

#### Parameter `RandomFillThreePass`

File contents are overwritten three times with random sequences before deletion. Three random sequences are transmitted over the network.

{% alert level="info" %}
Using this method makes sense only if the server stores data on a hard disk, and there is a risk of physical access by an attacker to the device.
{% endalert %}

#### Parameter `Discard`

An optimized cleanup method that uses file system capabilities for working with solid-state drives. File contents are marked as free through the `falloc` system call with the `FALLOC_FL_PUNCH_HOLE` flag. The file system will free blocks fully used by the file through the `blkdiscard` call, and the remaining space will be overwritten with zeros.

Advantages of the `Discard` method:

- Traffic volume does not depend on file size, only on their quantity.
- Can ensure inaccessibility of old data in some server configurations.
- Works for both hard drives and solid-state drives.
- Allows increasing solid-state drive lifespan.

## Changing NFS Server Parameters for Created PersistentVolumes

Changing NFS server parameters for created PersistentVolumes is impossible, as connection data to the NFS server is stored directly in the PersistentVolume manifest and cannot be changed. Changing NFSStorageClass will also not affect connection settings in existing PVs.

## Creating Volume Snapshots

In `csi-nfs`, snapshots are created by archiving the volume folder. The archive is saved in the root of the NFS server folder specified in the [`spec.connection.share`](/modules/csi-nfs/cr.html#nfsstorageclass-v1alpha1-spec-connection-share) parameter. When creating volume snapshots, it's important to consider the [requirements and limitations](#volume-snapshot-limitations) listed above.

To create a snapshot, perform the following steps:

1. Enable the [`snapshot-controller`](/modules/snapshot-controller/) module:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: snapshot-controller
   spec:
     enabled: true
     version: 1
   EOF
   ```

1. Create a volume snapshot, specifying the required parameters:

   ```shell
   d8 k apply -f - <<EOF
   apiVersion: snapshot.storage.k8s.io/v1
   kind: VolumeSnapshot
   metadata:
     name: my-snapshot
     namespace: <namespace name where the PVC is located>
   spec:
     volumeSnapshotClassName: csi-nfs-snapshot-class
     source:
       persistentVolumeClaimName: <PVC name for which the snapshot needs to be created>
   EOF
   ```

1. Check the status of the created snapshot:

   ```shell
   d8 k get volumesnapshot
   ```

## Issues with Deleting PVs with RPC-with-TLS Support

If the [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass) resource was configured with RPC-with-TLS support, a situation may arise where the PersistentVolume cannot be deleted. This happens due to secret deletion (e.g., after deleting [NFSStorageClass](/modules/csi-nfs/cr.html#nfsstorageclass)) that stores mounting parameters. As a result, the controller cannot mount the NFS folder to delete the `<PV name>` folder.

### Adding Multiple CAs to the `tlsParameters.ca` Parameter

To add multiple CA certificates to the `tlsParameters.ca` parameter, use the following commands:

**For two CAs:**

```shell
cat CA1.crt CA2.crt | base64 -w0
```

**For three CAs:**

```shell
cat CA1.crt CA2.crt CA3.crt | base64 -w0
```

**For more CAs:**

```shell
cat CA1.crt CA2.crt CA3.crt ... CAN.crt | base64 -w0
```
