---
title: "Snapshots"
permalink: en/virtualization-platform/documentation/user/resource-management/snapshots.html
---

Snapshots are designed to save the state of a resource at a particular point in time. Disk snapshots and virtual machine snapshots are currently supported.

## Creating disk snapshots

The `VirtualDiskSnapshot` resource is used to create snapshots of virtual disks. These snapshots can serve as a data source when creating new disks, such as for cloning or information recovery.

To ensure data integrity, a disk snapshot can be created in the following cases:

- The disk is not attached to any virtual machine.
- The VM is powered off.
- The VM is running, but qemu-guest-agent is installed in the guest OS.
  The file system has been successfully "frozen" (fsfreeze operation).

If data consistency is not required (for example, for test scenarios), a snapshot can be created:

- On a running VM without "freezing" the file system.
- Even if the disk is attached to an active VM.

To do this, specify in the VirtualDiskSnapshot manifest:

```yaml
spec:
  requiredConsistency: false
```

An example manifest for creating a disk snapshot:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDiskSnapshot
metadata:
  name: linux-vm-root-$(date +%s)
spec:
  requiredConsistency: true
  virtualDiskName: linux-vm-root
EOF
```

To view a list of disk snapshots, run the following command:

```bash
d8 k get vdsnapshot
```

Example output:

```console
NAME                     PHASE     CONSISTENT   AGE
linux-vm-root-1728027905   Ready                  3m2s
```

After creation, `VirtualDiskSnapshot` can be in the following states (phases):

- `Pending`: Waiting for all dependent resources required for snapshot creation to be ready.
- `InProgress`: The process of creating a virtual disk snapshot is in progress.
- `Ready`: Snapshot creation has been successfully completed and the virtual disk snapshot is available for use.
- `Failed`: An error occurred during the virtual disk snapshot creation process.
- `Terminating`: The resource is in the process of being deleted.

Diagnosing problems with a resource is done by analyzing the information in the `.status.conditions` block.

A full description of the `VirtualDiskSnapshot` resource configuration parameters for machines can be found at [link](/products/virtualization-platform/reference/cr/virtualdisksnapshot.html).

How to create a disk image in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "Disk Images" section.
- Click "Create Disk Snapshot".
- In the "Disk Snapshot Name" field, enter a name for the snapshot.
- On the "Configuration" tab, in the "Disk Name" field, select the disk from which the snapshot will be created.
- Enable the "Consistency Guarantee" switch.
- Click the "Create" button.
- The image status is displayed at the top left, under the snapshot name.

## Recovering disks from snapshots

In order to restore a disk from a previously created disk snapshot, you must specify a corresponding object as `dataSource`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  persistentVolumeClaim:
    size: 10Gi
    # Substitute your StorageClass name.
    storageClassName: i-sds-replicated-thin-r2
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-1728027905
EOF
```

How to restore a disk from a previously created snapshot in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "VM Disks" section.
- Click "Create Disk""
- In the form that opens, enter a name for the disk in the "Disk Name" field.
- In the "Source" field, make sure the "Snapshots" checkbox is selected.
- From the drop-down list, select the disk snapshot you want to restore from.
- In the "Size" field, set a size that is the same or larger than the size of the original disk.
- In the "StorageClass Name" field, enter the "StorageClass" of the original disk.
- Click the "Create" button.
- The disk status is displayed at the top left, under the disk name.

## Creating snapshots of virtual machines

A virtual machine snapshot is a saved state of a virtual machine at a specific point in time. The `VirtualMachineSnapshot` resource is used to create virtual machine snapshots.

{% alert level="warning" %}
It is recommended to disconnect all images (VirtualImage/ClusterVirtualImage) from the virtual machine before creating its snapshot. Disk images are not saved together with the VM snapshot, and their absence in the cluster during recovery may cause the virtual machine to fail to start and remain in a `Pending` state while waiting for the images to become available.
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

### Creating snapshots

Creating a virtual machine snapshot will fail if at least one of the following conditions is met:

- not all dependencies of the virtual machine are ready;
- there are changes pending restart of the virtual machine;
- there is a disk in the process of resizing among the dependent devices.

When a snapshot is created, the dynamic IP address of the VM is automatically converted to a static IP address and saved for recovery.

If you do not want to convert and use the old IP address of the virtual machine, you can set the corresponding policy to `Never`. In this case, the address type without conversion (`Auto` or `Static`) will be used.

```yaml
spec:
  keepIPAddress: Never
```

An example manifest to create a snapshot of a virtual machine:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshot
metadata:
  name: linux-vm-snapshot
spec:
  virtualMachineName: linux-vm
  requiredConsistency: true
  keepIPAddress: Never
EOF
```

After successfully creating a snapshot, its status will show the list of resources saved in the snapshot.

Output example:

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
- Go to the "Virtualization" → "Virtual Machines" section.
- Select the required VM from the list and click on its name.
- Go to the "Snapshots" tab.
- Click the "Create" button.
- In the form that opens, enter `linux-vm-snapshot` in the "Snapshot name" field.
- On the "Configuration" tab, select `Never` in the "IP address conversion policy" field.
- Enable the "Consistency Guarantee" switch.
- In the "Snapshot Storage Class" field, select a class for the disk snapshot.
- Click the "Create" button.
- The snapshot status is displayed at the top left, under the snapshot name.

Restore a virtual machine

To restore a VM from a snapshot, use the `VirtualMachineOperation` resource with the `restore` type.

Example:

```yaml
apiVersion: virtualisation.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: restore-vm
spec:
  type: Restore
  virtualMachineName: <name of the VM to be restored>
  restore:
    mode: DryRun | Strict | BestEffort
    virtualMachineSnapshotName: <name of the VM snapshot from which to restore>
```

One of three modes can be used for this operation:

- `DryRun`: Idle run of the restore operation, used to check for possible conflicts, which will be displayed in the resource status (`status.resources`).
- `Strict`: Strict recovery mode, used when the VM must be restored exactly as captured in the snapshot; missing external dependencies may cause the VM to remain in `Pending` status after recovery.
- `BestEffort`: Missing external dependencies (`ClusterVirtualImage`, `VirtualImage`) are ignored and removed from the VM configuration.

Restoring a virtual machine from a snapshot is only possible if all the following conditions are met:

- The VM to be restored exists in the cluster (the `VirtualMachine` resource exists and its `.metadata.uid` matches the identifier used when creating the snapshot).
- The disks to be restored (identified by name) are either not attached to other VMs or do not exist in the cluster.
- The IP address to be restored is either not used by any other VM or does not exist in the cluster.
- The MAC addresses to be restored are either not used by any other VMs or do not exist in the cluster.

{% alert level="warning" %}
If some resources on which the VM depends (for example, `VirtualMachineClass`, `VirtualImage`, `ClusterVirtualImage`) are missing from the cluster but existed when the snapshot was taken, the VM will remain in the `Pending` state after recovery.
In this case, you must manually edit the VM configuration to update or remove the missing dependencies.
{% endalert %}

You can view information about conflicts when restoring a VM from a snapshot in the resource status:

```bash
d8 k get vmop <vmop-name> -o json | jq “.status.resources”
```

{% alert level="warning" %}
It is not recommended to cancel the restore operation (delete the `VirtualMachineOperation` resource in the `InProgress` phase) from a snapshot, which can result in an inconsistent state of the restored virtual machine.
{% endalert %}

## Data export

DVP allows you to export virtual machine disks and disk images using the `d8` utility (version 1.17 and above).

Example: export a disk (run on a cluster node):

```bash
d8 download -n <namespace> vd/<virtual-disk-name> -o file.img
```

Example: export a disk snapshot (run on a cluster node):

```bash
d8 download -n <namespace> vds/<virtual-disksnapshot-name> -o file.img
```

To export resources outside the cluster, you must also use the `--publish` flag.
