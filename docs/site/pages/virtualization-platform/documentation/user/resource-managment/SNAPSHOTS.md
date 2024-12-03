---
title: "Snapshots"
permalink: en/virtualization-platform/documentation/user/resource-management/snapshots.html
---

Snapshots are used to save the state of a resource at a specific point in time. Both disk and virtual machine snapshots are supported.

## Creating snapshots from disks

To create disk snapshots, the [VirtualDiskSnapshot](../../../reference/cr/virtualdisksnapshot.html) resource is used. It can be used as a data source to create new virtual disks.

To ensure the integrity and consistency of the data, a disk snapshot can be created under the following conditions:

- The virtual disk is not attached to any virtual machine.
- The virtual disk is attached to a virtual machine that is powered off.
- The virtual disk is attached to a running virtual machine, and the guest agent (`qemu-guest-agent`) is installed in the virtual machine's OS, and the filesystem freeze operation was successful.

If data integrity and consistency are not critical, a snapshot can be taken from a running virtual machine without freezing the filesystem. In this case, add the following to the `VirtualDiskSnapshot` resource specification:

```yaml
spec:
  requiredConsistency: false
```

When creating a snapshot, you need to specify the name of the `VolumeSnapshotClass` that will be used to create the snapshot.

To get a list of supported `VolumeSnapshotClass` resources, run the following command:

```shell
d8 k get volumesnapshotclasses
# NAME                     DRIVER                                DELETIONPOLICY   AGE
# csi-nfs-snapshot-class   nfs.csi.k8s.io                        Delete           34d
# sds-replicated-volume    replicated.csi.storage.deckhouse.io   Delete           39d
```

Here is an example manifest for creating a disk snapshot:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDiskSnapshot
metadata:
  name: linux-vm-root-snapshot
spec:
  requiredConsistency: true
  virtualDiskName: linux-vm-root
  volumeSnapshotClassName: sds-replicated-volume
EOF
```

To view the list of disk snapshots, run the following command:

```shell
d8 k get vdsnapshot
# NAME                   PHASE     CONSISTENT   AGE
# linux-vm-root-snapshot Ready     true         3m2s
```

After creation, the `VirtualDiskSnapshot` resource can be in the following states:

- `Pending` — Waiting for the readiness of all dependent resources required to create the snapshot.
- `InProgress` — The process of creating the virtual disk snapshot is ongoing.
- `Ready` — The snapshot creation has been successfully completed, and the virtual disk snapshot is available for use.
- `Failed` — An error occurred during the creation process of the virtual disk snapshot.
- `Terminating` — The resource is in the process of being deleted.

## Restoring disks from snapshots

To restore a disk from a previously created disk snapshot, you need to specify the corresponding object as the `dataSource`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Disk storage settings.
  persistentVolumeClaim:
    # Specify a size greater than the snapshot size.
    size: 10Gi
    # Replace with your StorageClass name.
    storageClassName: i-linstor-thin-r2
  # Data source from which the disk is created.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-snapshot
EOF
```

## Creating Virtual Machine Snapshots

To create snapshots of virtual machines, the [VirtualMachineSnapshot](../../../reference/cr/virtualmachinesnapshot.html) resource is used.

To ensure data integrity and consistency, a virtual machine snapshot will be created if at least one of the following conditions is met:

- The virtual machine is powered off.
- The `qemu-guest-agent` is installed in the virtual machine's operating system, and the file system freeze operation was successful.

If integrity and consistency are not critical, a snapshot can be taken from a running virtual machine without freezing the file system. To do this, specify the following in the [VirtualMachineSnapshot](../../../reference/cr/virtualmachinesnapshot.html) resource's specification:

```yaml
spec:
  requiredConsistency: false
```

When creating a snapshot, you need to specify the names of the volume snapshot classes `VolumeSnapshotClass`, which will be used to create snapshots of the volumes attached to the virtual machine.

To get a list of supported `VolumeSnapshotClass` resources, run the following command:

```shell
d8 k get volumesnapshotclasses
# NAME                     DRIVER                                DELETIONPOLICY   AGE
# csi-nfs-snapshot-class   nfs.csi.k8s.io                        Delete           34d
# sds-replicated-volume    replicated.csi.storage.deckhouse.io   Delete           39d
```

A virtual machine snapshot will not be created if any of the following conditions are met:

- Not all dependent devices of the virtual machine are ready.
- There are changes waiting for a virtual machine restart.
- One of the dependent devices is a disk that is in the process of resizing.

When creating a virtual machine snapshot, the IP address will be converted to static and will be used later when restoring the virtual machine from the snapshot.

If converting and using the old IP address of the virtual machine is not required, you can set the corresponding policy to `Never`. In this case, the address type without conversion (`Auto` or `Static`) will be used.

```yaml
spec:
  keepIPAddress: Never
```

Example manifest for creating a snapshot of a virtual machine:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshot
metadata:
  name: linux-vm-snapshot
spec:
  virtualMachineName: linux-vm
  volumeSnapshotClasses:
    - storageClassName: i-linstor-thin-r2 # Replace with your StorageClass name.
      volumeSnapshotClassName: sds-replicated-volume # Replace with your VolumeSnapshotClass name.
  requiredConsistency: true
  keepIPAddress: Never
EOF
```

## Restoring virtual machines from snapshots

To restore virtual machines from snapshots, the [VirtualMachineRestore](../../../reference/cr/virtualmachinerestore.html) resource is used.

During the restoration process, a new virtual machine will be created, along with all its dependent resources (disks, IP address, automation script resource (Secret), and resources for dynamically attaching disks [VirtualMachineBlockDeviceAttachment](../../../reference/cr/virtualmachineblockdeviceattachment.html)).

If a name conflict occurs between existing and restoring resources for [VirtualMachine](../../../reference/cr/virtualmachine.html), [VirtualDisk](../../../reference/cr/virtualdisk.html), or [VirtualMachineBlockDeviceAttachment](../../../reference/cr/virtualmachineblockdeviceattachment.html), the restoration will fail. To avoid this, use the `nameReplacements` parameter.

If the restoring resource [VirtualMachineIPAddress](../../../reference/cr/virtualmachineipaddress.html) already exists in the cluster, it should not be attached to another virtual machine. Additionally, if it is a `Static` resource, its IP address must match. The restored automation secret should also match the restored one completely. Failure to meet these conditions will result in a restoration failure.

Example manifest for restoring a virtual machine from a snapshot:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: linux-vm-restore
spec:
  virtualMachineSnapshotName: linux-vm-snapshot
  nameReplacements:
    - from:
        kind: VirtualMachine
        name: linux-vm
      to: linux-vm-2 # Recreate the existing virtual machine linux-vm with a new name linux-vm-2.
    - from:
        kind: VirtualDisk
        name: linux-vm-root
      to: linux-vm-root-2 # Recreate the existing virtual disk linux-vm-root with a new name linux-vm-root-2.
    - from:
        kind: VirtualDisk
        name: blank-disk
      to: blank-disk-2 # Recreate the existing virtual disk blank-disk with a new name blank-disk-2.
    - from:
        kind: VirtualMachineBlockDeviceAttachment
        name: attach-blank-disk
      to: attach-blank-disk-2 # Recreate the existing virtual machine block device attachment attach-blank-disk with a new name attach-blank-disk-2.
EOF
```
