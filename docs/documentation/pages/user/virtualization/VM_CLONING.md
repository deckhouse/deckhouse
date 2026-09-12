---
title: "Cloning virtual machines"
permalink: en/user/virtualization/vm-cloning.html
description: "Cloning a virtual machine: creating a copy of a running machine and restoring a copy from a snapshot with renamed and adjusted parameters."
search: VM cloning, virtual machine clone, nameReplacements
---

A virtual machine clone is created either from an existing VM or from a previously created snapshot of that machine.

{% alert level="warning" %}
The cloned VM gets a new IP address for the cluster network and new MAC addresses for the additional network interfaces (if there are any), so after cloning you have to reconfigure the network parameters of the guest OS.
{% endalert %}

{% alert level="info" %}
Labels aren't copied from the source VM to the clone. This prevents Service traffic (Services select VMs by labels) from being routed to the clone. If the clone has to be part of a Service, add the labels you need after cloning. For example:

```bash
d8 k label vm <VM_NAME> label-name=label-value
```

{% endalert %}

Cloning creates a copy of a VM, so the resources of the new VM have to have unique names. The `nameReplacements` and `customization` parameters are used for this:

- `nameReplacements`: Lets you replace the names of existing resources with new ones to avoid conflicts.
- `customization`: Sets a prefix or a suffix for the names of all cloned VM resources (disks, IP addresses, and so on).

Here is an example of renaming specific resources:

```yaml
nameReplacements:
  - from:
      kind: VirtualMachine
      name: <OLD_VM_NAME>
    to:
      name: <NEW_VM_NAME>
  - from:
      kind: VirtualDisk
      name: <OLD_DISK_NAME>
    to:
      name: <NEW_DISK_NAME>
  ...
```

As a result, a VM named `<NEW_VM_NAME>` is created, and the specified resources are renamed according to the replacement rules.

Here is an example of adding a prefix or a suffix to all resources:

```yaml
customization:
  namePrefix: <PREFIX>
  nameSuffix: <SUFFIX>
```

As a result, a VM named `<PREFIX><ORIGINAL_VM_NAME><SUFFIX>` is created, and all resources (disks, IP addresses, and so on) get the prefix and the suffix.

You can use one of three modes for the cloning operation:

- `DryRun`: A test run to check for possible conflicts. The results are shown in the `status.resources` field of the corresponding operation resource.
- `Strict`: The strict mode, which requires all resources with new names and their dependencies (for example, images) to be present in the VM being cloned.
- `BestEffort`: The mode in which missing external dependencies (for example, [ClusterVirtualImage](/modules/virtualization/cr.html#clustervirtualimage), [VirtualImage](/modules/virtualization/cr.html#virtualimage)) are automatically removed from the configuration of the VM being cloned.

To view information about the conflicts that arose during cloning, check the status of the operation resource:

```bash
# For cloning from an existing VM.
d8 k get vmop <VMOP_NAME> -o json | jq '.status.resources'

# For cloning from a VM snapshot.
d8 k get vmsop <VMSOP_NAME> -o json | jq '.status.resources'
```

## Creating a clone of an existing VM

A clone is assembled from temporary snapshots of a machine, so you don't have to stop it.

{% tabs vm-clone %}

{% tab "Using the CLI" %}

A VM is cloned using the [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource with the `Clone` operation type.

Cloning is supported both for powered-off and for running virtual machines. When a running VM is cloned, a consistent snapshot is created automatically, and the clone is then built from it.

> Set the `.spec.runPolicy: AlwaysOff` parameter in the configuration of the VM being cloned to prevent the clone from starting automatically. This is because the clone inherits the behavior of the parent VM.

Before cloning, prepare the guest OS to avoid conflicts of unique identifiers and network settings.

Linux:

- clear `machine-id` with the `sudo truncate -s 0 /etc/machine-id` command (for systemd) or delete the `/var/lib/dbus/machine-id` file;
- delete the SSH host keys: `sudo rm -f /etc/ssh/ssh_host_*`;
- clear the network interface configurations (if static settings are used);
- clear the Cloud-Init cache (if it's used): `sudo cloud-init clean`.

Windows:

- run generalization with `sysprep` using the `/generalize` parameter, or use tools to clear unique identifiers (SID, hostname, and so on).

To create a VM clone, use the following resource:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: <VMOP_NAME>
spec:
  type: Clone
  virtualMachineName: <name of the VM to be cloned>
  clone:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

The `nameReplacements` and `customization` parameters are configured in the [`.spec.clone`](/modules/virtualization/cr.html#virtualmachineoperation-v1alpha2-spec-clone) block (general description above).

> During cloning, temporary snapshots are created automatically for the virtual machine and all its disks. The new VM is then assembled from these snapshots. After the cloning process finishes, the temporary snapshots are deleted automatically and you won't see them in the resource list. However, the specification of the cloned disks keeps a reference (`dataSource`) to the corresponding snapshot, even though the snapshot itself no longer exists. This is expected behavior and doesn't indicate a problem, because such references are valid: by the time the clone starts, all the necessary data has already been transferred to the new disks.

The following example shows cloning a VM named `database` and the `database-root` disk attached to it.

An example with renaming specific resources:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: clone-database
spec:
  type: Clone
  virtualMachineName: database
  clone:
    mode: Strict
    nameReplacements:
      - from:
          kind: VirtualMachine
          name: database
        to:
          name: database-clone
      - from:
          kind: VirtualDisk
          name: database-root
        to:
          name: database-clone-root
```

As a result, a VM named `database-clone` and a disk named `database-clone-root` are created.

An example with a prefix for all resources:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: clone-database
spec:
  type: Clone
  virtualMachineName: database
  clone:
    mode: Strict
    customization:
      namePrefix: clone-
      nameSuffix: -prod
```

As a result, a VM named `clone-database-prod` and a disk named `clone-database-root-prod` are created.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the virtual machine you need from the list and click the ellipsis button.
1. In the menu that opens, select **Clone**.
1. In the **Machine cloning** window that opens, select the snapshot to create the clone from in the **Virtual machine snapshot name** field. The clone is created from a snapshot, so prepare the snapshot in advance.
1. In the **Cloning mode** field, select `Strict` or `BestEffort`.
1. If required, set new names for the clone resources in the **Customization** → **Resource renaming** block, specifying the resource type, the original name, and the new name.
1. Click **Clone**.

{% endtab %}

{% endtabs %}

## Creating a clone from a VM snapshot

A VM is cloned from a snapshot using the [VirtualMachineSnapshotOperation](/modules/virtualization/cr.html#virtualmachinesnapshotoperation) resource with the `CreateVirtualMachine` operation type.

To create a VM clone from a snapshot, use the following resource:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: <VMSOP_NAME>
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: <name of the VM snapshot from which to clone>
  createVirtualMachine:
    mode: DryRun | Strict | BestEffort
    nameReplacements: []
    customization: {}
```

The `nameReplacements` and `customization` parameters are configured in the [`.spec.createVirtualMachine`](/modules/virtualization/cr.html#virtualmachinesnapshotoperation-v1alpha2-spec-createvirtualmachine) block (general description above).

To view the list of resources saved in a snapshot, run the following command:

```bash
d8 k get vmsnapshot <SNAPSHOT_NAME> -o jsonpath='{.status.resources}' | jq
```

{% alert level="info" %}
When a VM is cloned from a snapshot, the disks related to it are also created from the corresponding snapshots, so the disk specification contains the `dataSource` parameter with a reference to the disk snapshot needed.
{% endalert %}

The following example shows cloning from a VM snapshot named `database-snapshot`, which contains the `database` VM and the `database-root` disk.

An example with renaming specific resources:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: clone-database-from-snapshot
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: database-snapshot
  createVirtualMachine:
    mode: Strict
    nameReplacements:
      - from:
          kind: VirtualMachine
          name: database
        to:
          name: database-clone
      - from:
          kind: VirtualDisk
          name: database-root
        to:
          name: database-clone-root
```

As a result, a VM named `database-clone` and a disk named `database-clone-root` are created.

An example with a prefix for all resources:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineSnapshotOperation
metadata:
  name: clone-database-from-snapshot
spec:
  type: CreateVirtualMachine
  virtualMachineSnapshotName: database-snapshot
  createVirtualMachine:
    mode: Strict
    customization:
      namePrefix: clone-
      nameSuffix: -prod
```

As a result, a VM named `clone-database-prod` and a disk named `clone-database-root-prod` are created.
