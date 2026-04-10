---
title: "Snapshots"
permalink: en/virtualization-platform/documentation/user/resource-management/snapshots.html
---

Snapshots allow you to capture the current state of a resource for later recovery or cloning: a disk snapshot saves only the data from the selected disk, while a virtual machine snapshot includes the VM settings and the state of all its disks.

## Consistent snapshots

Snapshots can be consistent or inconsistent; this is controlled by the `requiredConsistency` parameter. By default, `requiredConsistency` is set to `true`, which means a consistent snapshot is required.

A consistent snapshot captures a complete and consistent state of disk data. You can create such a snapshot when one of the following conditions is met:

- The disk is not attached to any virtual machine — the snapshot will always be consistent.
- The virtual machine is turned off.
- [`qemu-guest-agent`](/products/virtualization-platform/documentation/user/resource-management/virtual-machines.html#guest-os-agent) is installed and running in the guest OS. When a snapshot is created, it temporarily suspends ("freezes") the file system to ensure consistency.

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
NAME                       PHASE     CONSISTENT   AGE
linux-vm-root-1728027905   Ready     true         3m2s
```

The `CONSISTENT` field indicates whether the snapshot is consistent (`true`) or not (`false`). This value is determined automatically based on the snapshot creation conditions and cannot be changed.

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
    storageClassName: rv-thin-r2
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
apiVersion: virtualization.deckhouse.io/v1alpha2
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

{% alert level="info" %}
Labels are not copied from the source VM to the clone. This prevents Service traffic (Services select VMs by labels) from being routed to the clone. If the clone should be part of a Service, add the required labels after cloning. For example:

```bash
d8 k label vm <vm-name> label-name=label-value
```

{% endalert %}

Cloning creates a copy of a VM, so the resources of the new VM must have unique names. To do this, use the `nameReplacements` and/or `customization` parameters:

- `nameReplacements`: Allows you to replace the names of existing resources with new ones to avoid conflicts.
- `customization`: Sets a prefix or suffix for the names of all cloned VM resources (disks, IP addresses, etc.).

Example of renaming specific resources:

```yaml
nameReplacements:
  - from:
      kind: VirtualMachine
      name: <old-vm-name>
    to:
      name: <new-vm-name>
  - from:
      kind: VirtualDisk
      name: <old-disk-name>
    to:
      name: <new-disk-name>
# ...
```

As a result, a VM named `<prefix><original-vm-name><suffix>` will be created, and all resources (disks, IP addresses, etc.) will receive the prefix and suffix.

Example of adding a prefix or suffix to all resources:

```yaml
customization:
  namePrefix: <prefix>
  nameSuffix: <suffix>
```

As a result, a VM named <prefix><new name><suffix> will be created.

One of three modes can be used for the cloning operation:

- `DryRun`: Test run to check for possible conflicts. The results are displayed in the `status.resources` field of the corresponding operation resource.
- `Strict`: Strict mode requiring all resources with new names and their dependencies (e.g., images) to be present in the cloned VM.
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

Cloning is supported for both powered-off and running virtual machines. When cloning a running VM, a consistent snapshot is automatically created, from which the clone is then formed.

{% alert level="info" %}
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

To create a VM clone, use the following resource:

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

The following example demonstrates cloning a VM named `database` with an attached disk `database-root`:

Example with renaming specific resources:

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

As a result, a VM named `database-clone` and a disk named `database-clone-root` will be created.

Example with using a prefix for all resources:

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

As a result, a VM named `clone-database-prod` and a disk named `clone-database-root-prod` will be created.

### Creating a clone from a VM snapshot

Cloning a VM from a snapshot is performed using the VirtualMachineSnapshotOperation resource with the `CreateVirtualMachine` operation type.

To create a VM clone from a snapshot, use the following resource:

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

To view the list of resources saved in a snapshot, use the command:

```bash
d8 k get vmsnapshot <snapshot-name> -o jsonpath='{.status.resources}' | jq
```

{% alert level="info" %}
When cloning a VM from a snapshot, the disks associated with it are also created from the corresponding snapshots, so the disk specification will contain a `dataSource` parameter with a reference to the required disk snapshot.
{% endalert %}

The following example demonstrates cloning from a VM snapshot named `database-snapshot`, which contains a VM `database` and a disk `database-root`:

Example with renaming specific resources:

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

As a result, a VM named `database-clone` and a disk named `database-clone-root` will be created.

Example with using a prefix for all resources:

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

As a result, a VM named `clone-database-prod` and a disk named `clone-database-root-prod` will be created.

## USB Devices

{% alert level="warning" %}
USB device passthrough is available only in the **Enterprise Edition (EE)** of the Deckhouse Virtualization Platform.
{% endalert %}

DVP supports USB device passthrough to virtual machines using DRA (Dynamic Resource Allocation). This section describes how to use USB devices with virtual machines.

### Overview

DVP provides two custom resources for managing USB devices:

- [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) (cluster-scoped) — represents a USB device discovered on a specific node. Created automatically by the DRA system when a USB device is detected on a node.
- [USBDevice](/modules/virtualization/cr.html#usbdevice) (namespace-scoped) — represents a USB device available for attachment to virtual machines in a given namespace.

### How It Works

USB device passthrough follows a defined lifecycle — from device discovery on a node to attachment to a virtual machine:

1. The DRA driver automatically discovers USB devices on cluster nodes and creates [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resources.

1. An administrator assigns a namespace to the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource by setting the `.spec.assignedNamespace` field. This makes the device available in that namespace.

1. After the namespace is assigned, the controller automatically creates a corresponding [USBDevice](/modules/virtualization/cr.html#usbdevice) resource in that namespace.

1. The [USBDevice](/modules/virtualization/cr.html#usbdevice) is attached to a virtual machine by adding it to the `.spec.usbDevices` field of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

### Quick Start

The following steps describe the minimal workflow for attaching a USB device to a virtual machine:

1. Connect the USB device to a cluster node.
1. Verify that a NodeUSBDevice resource has been created:

   ```bash
   d8 k get nodeusbdevice
   ```

1. Assign a namespace to the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) by setting `.spec.assignedNamespace`.

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: NodeUSBDevice
   metadata:
     name: logitech-webcam
   spec:
     assignedNamespace: my-project
   EOF
   ```

1. Verify that a corresponding [USBDevice](/modules/virtualization/cr.html#usbdevice) resource has been created in the target namespace:

   ```bash
   d8 k get usbdevice -n my-project
   ```

1. Add the device to the `.spec.usbDevices` field of a [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource and ensure that the VM is scheduled on the node where the USB device is physically connected.

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
   spec:
     # ... other VM settings ...
     usbDevices:
       - name: logitech-webcam
   EOF
   ```

### NodeUSBDevice

[NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource reflects the state of a physical USB device detected on a cluster node. It is a cluster-scoped resource that represents a physical USB device on a node. It is created automatically by the DRA system.

Example of viewing all discovered USB devices:

```bash
d8 k get nodeusbdevice
```

Example output:

```console
NAME                 NODE           READY   ASSIGNED   NAMESPACE   AGE
usb-flash-drive     node-1         True    False                  10m
logitech-webcam     node-2         True    True      my-project   15m
```

#### NodeUSBDevice Conditions

The status of a [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource is represented by a set of conditions that describe its availability and assignment state:

- **Ready**: Indicates whether the device is ready to use.
  - `Ready`: Device is ready to use.
  - `NotReady`: Device exists but is not ready.
  - `NotFound`: Device is absent on the host.

- **Assigned**: Indicates whether a namespace is assigned to the device.
  - `Assigned`: Namespace is assigned and USBDevice resource is created.
  - `Available`: No namespace is assigned for the device.
  - `InProgress`: Device connection to namespace is in progress.

#### Assigning a Namespace

Before a USB device can be attached to a virtual machine, it must be exposed to a specific namespace. To make a USB device available in a specific namespace, set the `.spec.assignedNamespace` field:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: NodeUSBDevice
metadata:
  name: logitech-webcam
spec:
  assignedNamespace: my-project
EOF
```

After assigning the namespace, a corresponding [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is automatically created in the specified namespace.

### USBDevice

Once a namespace is assigned to a [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice), a corresponding [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is created in automatically that namespace. It is a namespace-scoped resource that represents a USB device available for attachment to virtual machines within a given namespace.

Example of viewing USB devices in a namespace:

```bash
d8 k get usbdevice -n my-project
```

Example output:

```console
NAME               NODE     MANUFACTURER   PRODUCT              SERIAL       ATTACHED   AGE
logitech-webcam    node-2   Logitech       Webcam C920         ABC123456   False      10m
```

#### USBDevice Attributes

The [USBDevice](/modules/virtualization/cr.html#usbdevice) resource exposes detailed information about the physical USB device through its status fields. This attributes are available in `.status.attributes`:

- `vendorID`: USB vendor ID (hexadecimal format).
- `productID`: USB product ID (hexadecimal format).
- `bus`: USB bus number.
- `deviceNumber`: USB device number on the bus.
- `serial`: Device serial number.
- `manufacturer`: Device manufacturer name.
- `product`: Device product name.
- `name`: Device name.

#### USBDevice Conditions

The [USBDevice](/modules/virtualization/cr.html#usbdevice) resource provides status conditions that reflect its readiness and attachment state.

- **Ready**: Indicates whether the device is ready to use.
  - `Ready`: Device is ready to use.
  - `NotReady`: Device exists but is not ready.
  - `NotFound`: Device is absent on the host.

- **Attached**: Indicates whether the device is attached to a virtual machine.
  - `AttachedToVirtualMachine`: Device is attached to a VM.
  - `Available`: Device is available for attachment.
  - `NoFreeUSBIPPort`: Device is requested by a VM but cannot be attached because there are no free USBIP ports on the target node. In this case, `Attached=False`.

### Attaching USB Device to VM

After the [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is available in a namespace, it can be attached to a virtual machine. To attach a USB device to a virtual machine, add the device to the `.spec.usbDevices` field of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource specification:

```bash
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # ... other VM settings ...
  usbDevices:
    - name: logitech-webcam
EOF
```

After creating or updating the VM, the USB device will be attached to the specified virtual machine.

{% alert level="info" %}
The virtual machine must be running on the same node where the USB device is physically connected.
{% endalert %}

{% alert level="warning" %}
During VM migration, the USB device briefly disconnects and reconnects on the new node when the VM switches to it. If migration fails, the device will remain on the original node.
{% endalert %}

### Viewing USB Device Details

To view detailed information about a USB device:

```bash
d8 k describe nodeusbdevice <device-name>
```

Example output:

```console
Name:         logitech-webcam
Namespace:
Labels:       <none>
Annotations:  <none>
API Version:  virtualization.deckhouse.io/v1alpha2
Kind:         NodeUSBDevice
Metadata:
  Creation Timestamp:  2024-01-15T10:30:00Z
  Generation:          1
  UID:                 abc123-def456-ghi789
Spec:
  Assigned Namespace:  my-project
Status:
  Node Name:           node-2
  Attributes:
    Bus:               1
    Device Number:     2
    Manufacturer:      Logitech
    Name:              Webcam C920
    Product:           Webcam C920
    Product ID:        082d
    Serial:            ABC123456
    Vendor ID:         046d
  Conditions:
    Type:              Ready
    Status:            True
    Reason:            Ready
    Message:           Device is ready to use
    Type:              Assigned
    Status:            True
    Reason:            Assigned
    Message:           Namespace is assigned for the device
  Observed Generation: 1
```

{% alert level="info" %}
If a USB device is physically disconnected from the node, the `Attached` condition becomes `False`.  
Both `USBDevice` and `NodeUSBDevice` resources update their status conditions to indicate that the device is no longer present on the host.
{% endalert %}

### Requirements and Limitations

USB device passthrough has several operational requirements and limitations that must be considered before use:

- The DRA driver must be installed on nodes where USB devices are to be discovered.
- USB devices are forwarded to the VM node over the network using USBIP. The VM does not need to run on the same node where the device is physically connected. When connecting over the network, the following limitations on the number of devices and hub selection apply:
  - Node can attach at most 16 USB devices: up to 8 on the USB 2.0 hub and up to 8 on the USB 3.0 hub.
  - Hub is determined by the device speed and cannot be changed. A device that operates at USB 2.0 speed cannot be attached to the USB 3.0 hub, and vice versa.
- USB devices support hot-plug — they can be attached to and detached from a running VM without stopping it.
- USB device passthrough requires proper kernel modules on the node.

## Data export

You can export virtual machine disks and disk snapshots using the `d8` utility (version 0.20.7 and above). For this function to work, the module [`storage-volume-data-manager`](/modules/storage-volume-data-manager/) must be enabled.

{% alert level="warning" %}
The disk must not be in use at the time of export. If it is attached to a VM, that VM must be stopped first.
{% endalert %}

Example: export a disk (run on a cluster node):

```bash
d8 data export download -n <namespace> vd/<virtual-disk-name> -o file.img
```

Example: export a disk snapshot (run on a cluster node):

```bash
d8 data export download -n <namespace> vd/<virtual-disk-name> -o file.img
```

If you are exporting data from a machine other than a cluster node (for example, from your local machine), use the `--publish` flag.

{% alert level="warning" %}
To import a downloaded disk back into the cluster, upload it as an [image](#load-an-image-from-the-command-line) or as a [disk](#upload-a-disk-from-the-command-line).
{% endalert %}
