---
title: "Snapshots"
permalink: en/virtualization-platform/documentation/user/resource-management/snapshots.html
---

Snapshots are used to save the state of a resource at a specific point in time. Both disk and virtual machine snapshots are supported.

## Creating disk snapshots

The [VirtualDiskSnapshot](../../../reference/cr/virtualdisksnapshot.html) resource is used to create snapshots of virtual disks. These snapshots can serve as a data source when creating new disks, such as for cloning or information recovery.

To ensure data integrity, a disk snapshot can be created in the following cases:

- The disk is not attached to any virtual machine.
- The VM is powered off.
- The VM is running, but qemu-guest-agent is installed in the guest OS.
The file system has been successfully “frozen” (fsfreeze operation).

If data consistency is not required (for example, for test scenarios), a snapshot can be created:

- On a running VM without “freezing” the file system.
- Even if the disk is attached to an active VM.

To do this, specify in the VirtualDiskSnapshot manifest:

```yaml
spec:
  requiredConsistency: false
```

When creating a snapshot, you need to specify the name of the `VolumeSnapshotClass` that will be used to create the snapshot.

To get a list of supported `VolumeSnapshotClass` resources, run the following command:

```shell
d8 k get volumesnapshotclasses
```

Example output:

```console
NAME                     DRIVER                                DELETIONPOLICY   AGE
csi-nfs-snapshot-class   nfs.csi.k8s.io                        Delete           34d
sds-replicated-volume    replicated.csi.storage.deckhouse.io   Delete           39d
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
```

Example output:

```console
NAME                     PHASE     CONSISTENT   AGE
inux-vm-root-1728027905  Ready     true         3m2s
```

After creation, the `VirtualDiskSnapshot` resource can be in the following states:

- `Pending` — Waiting for the readiness of all dependent resources required to create the snapshot.
- `InProgress` — The process of creating the virtual disk snapshot is ongoing.
- `Ready` — The snapshot creation has been successfully completed, and the virtual disk snapshot is available for use.
- `Failed` — An error occurred during the creation process of the virtual disk snapshot.
- `Terminating` — The resource is in the process of being deleted.

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block.

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
    storageClassName: i-sds-replicated-thin-r2
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

Snapshots can be used to realize the following scenarios:

- [Creating disk snapshots](#creating-disk-snapshots)
- [Restoring disks from snapshots](#restoring-disks-from-snapshots)
- [Creating Virtual Machine Snapshots](#creating-virtual-machine-snapshots)
- [Restore from snapshots](#restore-from-snapshots)
  - [Restore a virtual machine](#restore-a-virtual-machine)
  - [Creating a VM clone / Using a VM snapshot as a template for creating a VM](#creating-a-vm-clone--using-a-vm-snapshot-as-a-template-for-creating-a-vm)

![Creating Virtual Machine Snapshots](/../../../../images/virtualization-platform/vm-restore-clone.png)

If you plan to use the snapshot as a template, perform the following steps in the guest OS before creating it:

- Deleting personal data (files, passwords, command history).
- Install critical OS updates.
- Clearing system logs.
- Reset network settings.
- Removing unique identifiers (e.g. via `sysprep` for Windows).
- Optimizing disk space.
- Resetting initialization configurations (`cloud-init clean`).

{% alert level="info" %}
A snapshot contains the configuration of the virtual machine and snapshots of all its disks.
Restoring a snapshot assumes that the virtual machine is fully restored to the time when the snapshot was created.
{% endalert %}

The snapshot will be created successfully if:

- The VM is shut down
- `qemu-guest-agent` is installed and the file system is successfully “frozen”.
If data integrity is not critical, the snapshot can be created on a running VM without freezing the file system. To do this, specify in the specification:

```yaml
spec:
  requiredConsistency: false
```

When creating a snapshot, you need to specify the names of the volume snapshot classes `VolumeSnapshotClass`, which will be used to create snapshots of the volumes attached to the virtual machine.

To get a list of supported `VolumeSnapshotClass` resources, run the following command:

```shell
d8 k get volumesnapshotclasses
```

Example output:

```console
NAME                     DRIVER                                DELETIONPOLICY   AGE
csi-nfs-snapshot-class   nfs.csi.k8s.io                        Delete           34d
sds-replicated-volume    replicated.csi.storage.deckhouse.io   Delete           39d
```

A virtual machine snapshot will not be created if any of the following conditions are met:

- Not all dependent devices of the virtual machine are ready.
- There are changes waiting for a virtual machine restart.
- One of the dependent devices is a disk that is in the process of resizing.

When a snapshot is created, the dynamic IP address of the VM is automatically converted to a static IP address and saved for recovery.

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
    - storageClassName: i-sds-replicated-thin-r2 # Substitute your StorageClass name.
      volumeSnapshotClassName: sds-replicated-volume # Replace with your VolumeSnapshotClass name.
  requiredConsistency: true
  keepIPAddress: Never
EOF
```

## Restore from snapshots

The [VirtualMachineRestore](../../../reference/cr/virtualmachinerestore.html) resource is used to restore a virtual machine from a snapshot. During the restore process, the following objects are automatically created in the cluster:

- VirtualMachine - the main VM resource with the configuration from the snapshot.
- VirtualDisk - disks connected to the VM at the moment of snapshot creation.
- VirtualBlockDeviceAttachment - disk connections to the VM (if they existed in the original configuration).
- Secret - secrets with cloud-init or sysprep settings (if they were involved in the original VM).

Important: resources are created only if they were present in the VM configuration at the time the snapshot was created. This ensures that an exact copy of the environment is restored, including all dependencies and settings.

### Restore a virtual machine

{% alert level="warning" %}
To restore a virtual machine, you must delete its current configuration and all associated disks. This is because the restore process returns the virtual machine and its disks to the state that was fixed at the time the backup snapshot was created.
{% endalert %}

Example manifest for restoring a virtual machine from a snapshot:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: <restore name>
spec:
  virtualMachineSnapshotName: <virtual machine snapshot name>
EOF
```

### Creating a VM clone / Using a VM snapshot as a template for creating a VM

A snapshot of a virtual machine can be used both to create its exact copy (clone) and as a template for deploying new VMs with a similar configuration.
This requires creating a `VirtualMachineRestore` resource and setting the renaming parameters in the `.spec.nameReplacements` block to avoid name conflicts.

Example manifest for restoring a VM from a snapshot:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: <name>
spec:
  virtualMachineSnapshotName: <virtual machine snapshot name>
  nameReplacements:
    - From:
        kind: VirtualMachine
        name: <old vm name>
      to: <new vm name>
    - from:
        kind: VirtualDisk
        name: <old disk name>
      to: <new disk name>
    - from:
        kind: VirtualDisk
        name: <old secondary disk name>
      to: <new secondary disk name>
    - from:
        kind: VirtualMachineBlockDeviceAttachment
        name: <old attachment name>
      to: <new attachment name>
EOF
```

When restoring a virtual machine from a snapshot, it is important to consider the following conditions:

1. If the `VirtualMachineIPAddress` resource already exists in the cluster, it must not be assigned to another VM .
2. For static IP addresses (`type: Static`) the value must be exactly the same as what was captured in the snapshot.
3. Automation-related secrets (such as cloud-init or sysprep configuration) must exactly match the configuration being restored.

Failure to do so will result in a restore error . This is because the system checks the integrity of the configuration and the uniqueness of the resources to prevent conflicts in the cluster.
