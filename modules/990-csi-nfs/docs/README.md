---
title: "The csi-nfs module"
description: "The csi-nfs module: General Concepts and Principles."
---

The module provides CSI for managing NFS volumes and allows creating StorageClass in Kubernetes through [Custom Resources](./cr.html#nfsstorageclass) `NFSStorageClass`.

{% alert level="warning" %}
**Warning about using snapshots (Volume Snapshots)**

When creating snapshots of NFS volumes, it's important to understand their creation scheme and associated limitations. We recommend avoiding the use of snapshots in csi-nfs when possible:

1. The CSI driver creates a snapshot at the NFS server level.
2. For this, tar is used, which packages the volume contents, with all the limitations that may arise from this.
3. **Before creating a snapshot, be sure to stop the workload** (pods) using the NFS volume.
4. NFS does not ensure atomicity of operations at the file system level when creating a snapshot.
{% endalert %}

{% alert level="info" %}
For working with snapshots, the [snapshot-controller](../../snapshot-controller/) module must be connected.
{% endalert %}

{% alert level="info" %}
Creating a StorageClass for the CSI driver `nfs.csi.k8s.io` by the user is prohibited.
{% endalert %}

## System requirements and recommendations

### Requirements

- Use stock kernels provided with [supported distributions](https://deckhouse.io/documentation/v1/supported_versions.html#linux);
- Ensure the presence of a deployed and configured NFS server;
- To support RPC-with-TLS, enable `CONFIG_TLS` and `CONFIG_NET_HANDSHAKE` options in the Linux kernel.

### Recommendations

For module pods to restart when the `tlsParameters` parameter is changed in the module settings, the [pod-reloader](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/pod-reloader) module must be enabled (enabled by default).

## RPC-with-TLS mode limitations

- For the `mtls` security policy, only one client certificate is supported.
- A single NFS server cannot simultaneously operate in different security modes: `tls`, `mtls`, and standard (non-TLS) mode.
- The `tlshd` daemon must not be running on the cluster nodes, otherwise it will conflict with the daemon of our module. To prevent conflicts when enabling TLS, the third-party `tlshd` is automatically stopped on the nodes and its autostart is disabled.

## Quickstart guide

Note that all commands must be run on a machine that has administrator access to the Kubernetes API.

### Enabling module

1. Enable the `csi-nfs` module. This will result in the following actions across all cluster nodes:
   - registration of the CSI driver;
   - launch of service pods for the `csi-nfs` components.

   ```yaml
   kubectl apply -f - <<EOF
   apiVersion: deckhouse.io/v1alpha1
   kind: ModuleConfig
   metadata:
     name: csi-nfs
   spec:
     enabled: true
     version: 1
   EOF
   ```

2. Wait for the module to become `Ready`:

   ```shell
   kubectl get module csi-nfs -w
   ```

### Creating a StorageClass

To create a StorageClass, you need to use the [NFSStorageClass](./cr.html#nfsstorageclass) resource. Here is an example command to create such a resource:

```yaml
kubectl apply -f - <<EOF
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

A directory `<directory from share>/<PV name>` will be created for each PV.

### Checking module health

You can verify the functionality of the module using the instructions [in FAQ](./faq.html#how-to-check-module-health).

### Selects the method to clean the volume before deleting the PV

Files with user data may remain on the volume to be deleted. These files will be deleted and will not be accessible to other users via NFS.

However, the deleted files' data may be available to other clients if the server grants block-level access to its storage.

The `volumeCleanup` parameter will help you choose how to clean the volume before deleting it.

> **Caution.** This option does not affect files already deleted by the client application.

> **Caution.** This option affects only commands sent via the NFS protocol. The server-side execution of these commands is defined by:
>
> - NFS server service;
> - the file system;
> - the level of block devices and their virtualization (e.g. LVM);
> - the physical devices themselves.
>
> Make sure the server is trusted. Do not send sensitive data to servers that you are not sure of.

#### `SinglePass` method

Used if `volumeCleanup` is set to `RandomFillSinglePass`.

The contents of the files are overwritten with a random sequence before deletion. The random sequence is transmitted over the network.

#### `ThreePass` method

Used if `volumeCleanup` is set to `RandomFillThreePass`.

The contents of the files are overwritten three times with a random sequence before deletion. The three random sequences are transmitted over the network.

#### `Discard` method

Used if `volumeCleanup` is set to `Discard`.

Many file systems implement support for solid-state drives, allowing the space occupied by a file to be freed at the block level without writing new data to extend the life of the solid-state drive. However, not all solid-state drives guarantee that the freed block data is inaccessible.

If `volumeCleanup` is set to `Discard`, file contents are marked as free via the `falloc` system call with the `FALLOC_FL_PUNCH_HOLE` flag. The file system will free the blocks fully used by the file, via the `blkdiscard` call, and the remaining space will be overwritten with zeros.

Advantages of this method:

- the amount of traffic does not depend on the size of the files, only on the number of files;
- the method can make old data unavailable in some server configurations;
- works for both hard disks and SSDs;
- can maximize SSD lifetime.
