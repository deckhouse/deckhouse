---
title: "Snapshots"
permalink: en/virtualization-platform/documentation/user/resource-management/snapshots.html
---

Snapshots allow you to capture the current state of a resource for later recovery or cloning: a disk snapshot saves only the data from the selected disk, while a virtual machine snapshot includes the VM settings and the state of all its disks.

## Consistent snapshots

Snapshots can be consistent or inconsistent; this is controlled by the `requiredConsistency` parameter. By default, `requiredConsistency` is set to `true`, which means a consistent snapshot is required.

A consistent snapshot guarantees a consistent and complete state of the virtual machine's disks. Such a snapshot can be created when one of the following conditions is met:

- The virtual machine is turned off.
- [`qemu-guest-agent`](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#guest-os-agent) is installed in the guest system, which temporarily suspends the file system at the time the snapshot is created to ensure its consistency.

QEMU Guest Agent supports hook scripts that allow you to prepare applications for snapshot creation without stopping services, ensuring application-level consistency. For more information on configuring hooks scripts, see the [Guest OS agent](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#guest-os-agent) section.

An inconsistent snapshot may not reflect a consistent state of the virtual machine's disks and its components. Such a snapshot is created in the following cases:

- The VM is running, and `qemu-guest-agent` is not installed or not running in the guest OS.
- The snapshot manifest explicitly specifies the `requiredConsistency: false` parameter, and you want to avoid suspending the file system.

{% alert level="warning" %}
When restoring from such a snapshot, file system integrity issues may occur, as the data state may be inconsistent.
{% endalert %}

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

A full description of the `VirtualDiskSnapshot` resource configuration parameters for machines can be found at [link](/modules/virtualization/cr.html#virtualdisksnapshot).

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

### Creating snapshots

Creating a virtual machine snapshot will fail if at least one of the following conditions is met:

- Not all dependencies of the virtual machine are ready.
- There is a disk in the process of resizing among the dependent devices.

{% alert level="warning" %}
If there are pending VM changes awaiting a restart when the snapshot is created, the snapshot will include the updated VM configuration.
{% endalert %}

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

{% alert level="info" %}
When restoring a VM from a snapshot, the disks associated with it are also restored from the corresponding snapshots, so the disk specification will contain a `dataSource` parameter with a reference to the required disk snapshot.
{% endalert %}

## Creating a VM clone

You can create a VM clone in two ways: from an existing VM or from a previously created snapshot of that VM.

{% alert level="warning" %}
The cloned VM will be assigned a new IP address for the cluster network and MAC addresses for additional network interfaces (if any), so you will need to reconfigure the guest OS network settings after cloning.
{% endalert %}

Cloning creates a copy of a VM, so the resources of the new VM must have unique names. To do this, use the `nameReplacements` and/or `customization` parameters:

- `nameReplacements`: Allows you to replace the names of existing resources with new ones to avoid conflicts.
- `customization`: Sets a prefix or suffix for the names of all cloned VM resources (disks, IP addresses, etc.).

Configuration example:

```yaml
nameReplacements:
  - from:
      kind: <resource type>
      name: <old name>
    to:
      name: <new name>
customization:
  namePrefix: <prefix>
  nameSuffix: <suffix>
```

As a result, a VM named <prefix><new name><suffix> will be created.

One of three modes can be used for the cloning operation:

- `DryRun`: Test run to check for possible conflicts. The results are displayed in the `status.resources` field of the corresponding operation resource.
- `Strict`: Strict mode, requiring all resources with new names and their dependencies (e.g., images) to be present in the cloned VM.
- `BestEffort`: Mode in which missing external dependencies (e.g., ClusterVirtualImage, VirtualImage) are automatically removed from the configuration of the cloned VM.

Information about conflicts that arose during cloning can be viewed in the operation resource status:

```bash
# For cloning from an existing VM.
d8 k get vmop <vmop-name> -o json | jq '.status.resources'
# For cloning from a VM snapshot.
d8 k get vmsop <vmsop-name> -o json | jq '.status.resources'
```

### Creating a clone from an existing VM

VM cloning is performed using the VirtualMachineOperation resource with the `Clone` operation type.

{% alert level="warning" %}
Before cloning, the source VM must be [powered off](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#virtual-machine-startup-policy-and-virtual-machine-state-management).

It is recommended to set the `.spec.runPolicy: AlwaysOff` parameter in the configuration of the VM being cloned if you want to prevent the VM clone from starting automatically. This is because the clone inherits the behaviour of the parent VM.
{% endalert %}

Before cloning, you need to prepare the guest OS to avoid conflicts with unique identifiers and network settings.

Linux:

- Clear the `machine-id` using `sudo truncate -s 0 /etc/machine-id` (for systemd) or delete the `/var/lib/dbus/machine-id` file.
- Remove SSH host keys: `sudo rm -f /etc/ssh/ssh_host_*`.
- Clear network interface configuration (if static settings are used).
- Clear the Cloud-Init cache (if used): `sudo cloud-init clean`.

Windows:

- Run `sysprep` with the `/generalize` option, or use tools to reset unique identifiers (SID, hostname, etc.).

Example of creating a VM clone:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: <vmop-name>
spec:
  type: Clone
  virtualMachineName: <name of the VM to be cloned>
  clone:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

The `nameReplacements` and `customization` parameters are configured in the `.spec.clone` block (see [general description](#creating-a-vm-clone) above).

{% alert level="info" %}
During cloning, temporary snapshots are automatically created for the virtual machine and all its disks. The new VM is then assembled from these snapshots. After cloning is complete, the temporary snapshots are automatically deleted, so they are not visible in the resource list. However, the specification of cloned disks still contains a reference (`dataSource`) to the corresponding snapshot, even if the snapshot itself no longer exists. This is expected behavior and does not indicate a problem: such references remain valid because, by the time the clone starts, all necessary data has already been transferred to the new disks.
{% endalert %}

### Creating a clone from a VM snapshot

Cloning a VM from a snapshot is performed using the VirtualMachineSnapshotOperation resource with the `CreateVirtualMachine` operation type.

Example of creating a VM clone from a snapshot:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: <vmsop-name>
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: <name of the VM snapshot from which to clone>
  createVirtualMachine:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

The `nameReplacements` and `customization` parameters are configured in the `.spec.createVirtualMachine` block (see [general description](#creating-a-vm-clone) above).

{% alert level="info" %}
When cloning a VM from a snapshot, the disks associated with it are also created from the corresponding snapshots, so the disk specification will contain a `dataSource` parameter with a reference to the required disk snapshot.
{% endalert %}

## Data export

You can export virtual machine disks and disk snapshots using the `d8` utility (version 0.20.7 and above). For this function to work, the module [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) must be enabled.

{% alert level="warning" %}
The disk must not be in use at the time of export. If it is attached to a VM, that VM must be stopped first.
{% endalert %}

Example: export a disk (run on a cluster node):

```bash
d8 data download -n <namespace> vd/<virtual-disk-name> -o file.img
```

Example: export a disk snapshot (run on a cluster node):

```bash
d8 data download -n <namespace> vds/<virtual-disksnapshot-name> -o file.img
```

If you are exporting data from a machine other than a cluster node (for example, from your local machine), use the `--publish` flag.

{% alert level="warning" %}
To import a downloaded disk back into the cluster, upload it as an [image](#load-an-image-from-the-command-line) or as a [disk](#upload-a-disk-from-the-command-line).
{% endalert %}
