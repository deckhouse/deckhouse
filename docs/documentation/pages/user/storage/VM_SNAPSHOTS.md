---
title: "Snapshots of disks and virtual machines"
permalink: en/user/storage/vm-snapshots.html
description: "Snapshots of disks and virtual machines: data consistency, creating snapshots, and restoring from them."
search: snapshots, VirtualDiskSnapshot, VirtualMachineSnapshot, restore
---

Snapshots let you capture the current state of a resource for later recovery or [cloning](../virtualization/vm-cloning.html). A disk snapshot saves only the data of the selected disk, while a virtual machine snapshot includes the VM parameters and the state of all its disks.

## Consistent snapshots

Snapshots can be consistent or inconsistent. The `requiredConsistency` parameter is responsible for this, and its default value is `true`, which means that a consistent snapshot is required.

A consistent snapshot captures a coherent and integral state of the disk data. You can create such a snapshot when one of the following conditions is met:

- the disk isn't attached to any virtual machine, and then the snapshot is always consistent;
- the virtual machine is powered off;
- [`qemu-guest-agent`](../virtualization/vm-provisioning.html#guest-os-agent) is installed and running in the guest OS. When the snapshot is created, it temporarily pauses ("freezes") the file system to keep the data coherent.

An inconsistent snapshot may not reflect a coherent state of the virtual machine disks and its components. Such a snapshot is created if the VM is running and `qemu-guest-agent` isn't installed or isn't running in the guest OS.
If the snapshot manifest explicitly specifies `requiredConsistency: false` but `qemu-guest-agent` is running, an attempt to freeze the file system is still made so that the snapshot comes out consistent.

QEMU Guest Agent supports hook scripts that prepare applications for a snapshot without stopping services, keeping the state coherent at the application level. Configuring hook scripts is described in [Guest OS agent](../virtualization/vm-provisioning.html#guest-os-agent).

{% alert level="warning" %}
When recovering from such a snapshot, file system integrity problems are possible, because the data state may be incoherent.
{% endalert %}

## Creating disk snapshots

A disk snapshot saves the disk data at the moment of creation and serves as a source for new disks.

{% tabs snap-disk-create %}

{% tab "Using the CLI" %}

To create snapshots of virtual disks, use the [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) resource. These snapshots can serve as a data source when creating new disks, for example to clone or recover information.

To guarantee data integrity, you can create a disk snapshot in the following cases:

- The disk isn't attached to any virtual machine.
- The VM is powered off.
- The VM is running, but qemu-guest-agent is installed in the guest OS.
  The file system was successfully frozen (the fsfreeze operation).

If data consistency isn't required (for example, for test scenarios), you can create a snapshot:

- On a running VM without freezing the file system.
- Even if the disk is attached to an active VM.

To do this, specify the following in the [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) manifest:

```yaml
spec:
  requiredConsistency: false
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
EOF
```

To view the list of disk snapshots, run the following command:

```bash
d8 k get vdsnapshot
```

Example output:

```console
NAME                   PHASE     CONSISTENT   AGE
linux-vm-root-snapshot Ready     true         3m2s
```
{: .nowrap-default }

The `CONSISTENT` field with the `true` value means that the snapshot is consistent (`false`). The value is determined automatically from the snapshot creation conditions and can't be changed.

After creation, a [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) can be in the following states (phases):

- `Pending`: Waiting for all dependent resources required to create the snapshot to become ready.
- `InProgress`: The virtual disk snapshot is being created.
- `Ready`: The snapshot was created successfully and the virtual disk snapshot is available for use.
- `Failed`: An error occurred while creating the virtual disk snapshot.
- `Terminating`: The resource is being deleted.

The [`.status.conditions`](/modules/virtualization/cr.html#virtualdisksnapshot-v1alpha2-status-conditions) block shows the reason for a problem with the resource.

For a full description of the [VirtualDiskSnapshot](/modules/virtualization/cr.html#virtualdisksnapshot) resource configuration parameters, see [the resource documentation](/modules/virtualization/cr.html#virtualdisksnapshot).

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Disk snapshots**.
1. Click **Create**.
1. In the **Create resource** window that opens, enter the snapshot name in the **Name** field.
1. On the **Configuration** tab, select the disk to take the snapshot from in the **Virtual disk name** field.
1. Enable the **Required consistency** toggle.
1. Click **Apply**.
1. The snapshot status is shown in the **Status** column.

{% endtab %}

{% endtabs %}

## Recovering disks from snapshots

A new disk is created from a snapshot, and the original disk stays untouched.

{% tabs snap-disk-restore %}

{% tab "Using the CLI" %}

To recover a disk from a previously created disk snapshot, specify the corresponding object as the `dataSource`:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualDisk
metadata:
  name: linux-vm-root
spec:
  # Disk storage parameters.
  persistentVolumeClaim:
    # Specify a size larger than the value.
    size: 10Gi
    # Specify the name of your StorageClass.
    storageClassName: rv-thin-r2
  # The source the disk is created from.
  dataSource:
    type: ObjectRef
    objectRef:
      kind: VirtualDiskSnapshot
      name: linux-vm-root-snapshot
EOF
```

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Disks**.
1. Click **Create**.
1. In the form that opens, enter the disk name in the **Disk name** field.
1. In the **Source** field, select the disk snapshot you want to recover from in the drop-down list.
1. In the **Size** field, set a size equal to or larger than the size of the original disk.
1. In the **Storage class** field, select the StorageClass of the original disk.
1. Click **Create**.
1. The disk status is shown on its page.

{% endtab %}

{% endtabs %}

## Creating VM snapshots

A virtual machine snapshot is the saved state of a virtual machine at a certain point in time. To create virtual machine snapshots, use the [VirtualMachineSnapshot](/modules/virtualization/cr.html#virtualmachinesnapshot) resource.

{% alert level="warning" %}
Detach all images ([VirtualImage](/modules/virtualization/cr.html#virtualimage)/ClusterVirtualImage) from a virtual machine before taking its snapshot. Disk images aren't saved along with the VM snapshot, and their absence in the cluster during recovery can leave the virtual machine unable to start, in the Pending state, waiting for the image to become available.
{% endalert %}

{% tabs snap-vm-create %}

{% tab "Using the CLI" %}

Creating a virtual machine snapshot fails if at least one of the following conditions is met:

- not all dependent devices of the virtual machine are ready;
- one of the dependent devices is a disk that is being resized.

> **Important:** If the virtual machine has changes pending a restart at the moment the snapshot is taken, the updated configuration goes into the snapshot.

When a snapshot is created, the dynamic IP address of the VM is automatically converted to a static one and saved for recovery.

If you don't need the conversion and the reuse of the old virtual machine IP address, you can set the corresponding policy to `Never`. In that case, the address type is used without conversion (`Auto` or `Static`).

```yaml
spec:
  keepIPAddress: Never
```

Here is an example manifest for creating a virtual machine snapshot:

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

After the snapshot is created successfully, its status reflects the list of resources saved in the snapshot.

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

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. Go to the **Snapshots** tab.
1. Click **Add**.
1. In the form that opens, enter `linux-vm-snapshot` in the **Snapshot name** field.
1. Enable the **Integrity guarantee** toggle.
1. Click **Create**.
1. The snapshot status is shown on its page.
1. The created snapshots are listed on the **Snapshots** tab of the virtual machine, with the **Name**, **Status**, **Creation date**, and **Consistent** columns.

{% endtab %}

{% endtabs %}

## Recovering a VM

Recovery returns a machine and its disks to the state saved in a snapshot.

{% tabs snap-vm-restore %}

{% tab "Using the CLI" %}

Recovery is started by a [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource of the `restore` type:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: <VMOP_NAME>
spec:
  type: Restore
  virtualMachineName: <VM_NAME>
  restore:
    mode: DryRun | Strict | BestEffort
    virtualMachineSnapshotName: <VM_SNAPSHOT_NAME>
```

You can use one of three modes for this operation:

- `DryRun`: A dry run of the recovery operation, needed to check for possible conflicts, which are shown in the resource status (`status.resources`).
- `Strict`: The strict recovery mode, when the VM has to be recovered exactly as in the snapshot; missing external dependencies can leave the VM in `Pending` after recovery.
- `BestEffort`: Missing external dependencies ([ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), [VirtualImage](/modules/virtualization/cr.html#virtualimage)) are ignored and removed from the VM configuration.

Recovering a virtual machine from a snapshot is possible only when all of the following conditions are met:

- The VM being recovered is present in the cluster (the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource exists and its `.metadata.uid` matches the identifier used when the snapshot was created).
- The disks being recovered (identified by name) either aren't attached to other VMs or are absent from the cluster.
- The IP address being recovered either isn't taken by another VM or is absent from the cluster.
- The MAC addresses being recovered either aren't used by other VMs or are absent from the cluster.

> **Important:** If some resources the VM depends on (for example, [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass), [VirtualImage](/modules/virtualization/cr.html#virtualimage), [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage)) are absent from the cluster but existed at the moment the snapshot was created, the VM stays in the `Pending` state after recovery.
> In that case, edit the VM configuration manually and update or remove the missing dependencies.

To view information about conflicts when recovering a VM from a snapshot, check the resource status:

```bash
d8 k get vmop <VMOP_NAME> -o json | jq '.status.resources'
```

> **Important:** Don't cancel a recovery operation from a snapshot, that is, don't delete the [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource in the `InProgress` phase, because this can leave the virtual machine being recovered in an inconsistent state.

> When a VM is recovered from a snapshot, the disks related to it are also recovered from the corresponding snapshots, so the disk specification contains the `dataSource` parameter with a reference to the disk snapshot needed.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the virtual machine you need from the list and click the ellipsis button.
1. In the menu that opens, select **Restore**.
1. In the **Machine recovery** window that opens, select the snapshot in the **Virtual machine snapshot name** field.
1. In the **Recovery mode** field, select `Strict` or `BestEffort`.
1. Click **Restore**.

{% endtab %}

{% endtabs %}
