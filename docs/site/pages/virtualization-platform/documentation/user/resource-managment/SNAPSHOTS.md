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

How to restore a disk from a previously created snapshot in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" -> "VM Disks" section.
- Click "Create Disk""
- In the form that opens, enter a name for the disk in the "Disk Name" field.
- In the "Source" field, make sure the "Snapshots" checkbox is selected.
- From the drop-down list, select the disk snapshot you want to restore from.
- In the "Size" field, set a size that is the same or larger than the size of the original disk.
- In the "StorageClass Name" field, enter the "StorageClass" of the original disk.
- Click the "Create" button.
- The disk status is displayed at the top left, under the disk name.

## Creating Virtual Machine Snapshots

A virtual machine snapshot is a saved state of a virtual machine at a specific point in time. The [VirtualMachineSnapshot](../../../reference/cr/virtualmachinesnapshot.html) resource is used to create virtual machine snapshots.

{% alert level="warning" %}
It is recommended to disconnect all images (VirtualImage/ClusterVirtualImage) from the virtual machine before creating its snapshot. Disk images are not saved together with the VM snapshot, and their absence in the cluster during recovery may cause the virtual machine to fail to start and remain in a Pending state while waiting for the images to become available.
{% endalert %}

### Types of snapshots

Snapshots can be consistent or inconsistent, which is determined by the `requiredConsistency` parameter. By default, the `requiredConsistency` parameter is set to `true`, which requires a consistent snapshot.

A consistent snapshot guarantees a consistent and complete state of the virtual machine's disks. Such a snapshot can be created when one of the following conditions is met:

- The virtual machine is turned off.
- `qemu-guest-agent` is installed in the guest system, which temporarily suspends the file system at the time the snapshot is created to ensure its consistency.

  An inconsistent snapshot may not reflect the consistent state of the virtual machine's disks and its components. Such a snapshot is created in the following cases:

- The VM is running, and `qemu-guest-agent` is not installed or running in the guest OS.
- The VM is running, and `qemu-guest-agent` is not installed in the guest OS, but the snapshot manifest specifies the `requiredConsistency: false` parameter, and you want to avoid suspending the file system.

{% alert level="warning" %}
There is a risk of data loss or integrity violation when restoring from such a snapshot.
{% endalert %}

#### Scenarios for using snapshots

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
- Create a snapshot with a clear indication not to save the IP address: `keepIPAddress: Never`.

When creating an image, follow these recommendations:

- Disconnect all images if they were connected to the virtual machine.
- Do not use a static IP address for VirtualMachineIPAddress. If a static address has been used, change it to automatic.
- Create a snapshot with an explicit indication not to save the IP address: `keepIPAddress: Never`.

#### Creating snapshots

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

After successfully creating a snapshot, its status will show the list of resources saved in the snapshot.

Example output:

```yaml
status:
  ...
  resources:
  - apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualMachine
    name: linux-vm
  - apiVersion: v1
    kind: Secret
    name: cloud-init
  - apiVersion: virtualization.deckhouse.io/v1alpha2
    kind: VirtualDisk
    name: linux-vm-root
```

How to create a VM snapshot in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" -> "Virtual Machines" section.
- Select the required VM from the list and click on its name.
- Go to the "Snapshots" tab.
- Click the "Create" button.
- In the form that opens, enter `linux-vm-snapshot` in the "Snapshot name" field.
- On the "Configuration" tab, select `Never` in the "IP address conversion policy" field.
- Enable the "Consistency Guarantee" switch.
- In the "Snapshot Storage Class" field, select a class for the disk snapshot.
- Click the "Create" button.
- The snapshot status is displayed at the top left, under the snapshot name.

## Restore from snapshots

The [VirtualMachineRestore](../../../reference/cr/virtualmachinerestore.html) resource is used to restore a virtual machine from a snapshot. During the restore process, the following objects are automatically created in the cluster:

- VirtualMachine: The main VM resource with the configuration from the snapshot.
- VirtualDisk: Disks connected to the VM at the moment of snapshot creation.
- VirtualBlockDeviceAttachment: Disk connections to the VM (if they existed in the original configuration).
- VirtualMachineIPAddress: The IP address of the virtual machine (if the `keepIPAddress: Always` parameter was specified at the time of snapshot creation).
- Secret: Secrets with cloud-init or sysprep settings (if they were involved in the original VM).

Important: resources are created only if they were present in the VM configuration at the time the snapshot was created. This ensures that an exact copy of the environment is restored, including all dependencies and settings.

### Restore a virtual machine

There are two modes used for restoring a virtual machine. They are defined by the restoreMode parameter of the VirtualMachineRestore resource:

```yaml
spec:
  restoreMode: Safe | Forced
```

`Safe` is used by default.

{% alert level="warning" %}
To restore a virtual machine in `Safe` mode, you must delete its current configuration and all associated disks. This is because the restoration process returns the virtual machine and its disks to the state recorded at the snapshot's creation time.
{% endalert %}

The `Forced` mode is used to bring an already existing virtual machine to the state at the time of the snapshot.

{% alert level="warning" %}
`Forced` may disrupt the operation of the existing virtual machine because it will be stopped during restoration, and `VirtualDisks` and `VirtualMachineBlockDeviceAttachments` resources will be deleted for subsequent restoration.
{% endalert %}

Example manifest for restoring a virtual machine from a snapshot in `Safe` mode:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineRestore
metadata:
  name: <restore name>
spec:
  restoreMode: Safe
  virtualMachineSnapshotName: <virtual machine snapshot name>
EOF
```

### Creating a VM clone / Using a VM snapshot as a template for creating a VM

A snapshot of a virtual machine can be used both to create its exact copy (clone) and as a template for deploying new VMs with a similar configuration.
This requires creating a `VirtualMachineRestore` resource and setting the renaming parameters in the `.spec.nameReplacements` block to avoid name conflicts.

The list of resources and their names are available in the VM snapshot status in the `status.resources` block.

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

Failure to do so will result in a restore error, and the VirtualMachineRestore resource will enter the `Failed` state. This is because the system checks the integrity of the configuration and the uniqueness of the resources to prevent conflicts in the cluster.

When restoring or cloning a virtual machine, the operation may be successful, but the VM will remain in `Pending` state.

This occurs if the VM depends on resources (such as disk images or virtual machine classes) or their configurations that have been changed or deleted at the time of restoration.

Check the VM's conditions block using the command:

```bash
d8 k vm get <vmname> -o json | jq ‘.status.conditions’
```

Check the output for errors related to missing or changed resources. Manually update the VM configuration to remove dependencies that are no longer available in the cluster.
