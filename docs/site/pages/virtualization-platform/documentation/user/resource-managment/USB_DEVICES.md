---
title: "USB devices"
permalink: en/virtualization-platform/documentation/user/resource-management/usb-devices.html
---

{% alert level="warning" %}
USB device passthrough is available only in the Deckhouse Virtualization Platform **Enterprise Edition (EE)**.
{% endalert %}

DVP supports USB device passthrough to virtual machines using DRA (Dynamic Resource Allocation). This section describes how to use USB devices with virtual machines.

USB device passthrough requires:

- `containerd v2`: Detailed requirements for cluster nodes are described in the [`defaultCRI`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) parameter.
- [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) version 1.34 or higher.
- [Deckhouse Kubernetes Platform (DKP)](https://releases.deckhouse.io/) version 1.75 or higher.

## Overview

DVP provides two custom resources for managing USB devices:

- [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) (cluster-wide resource) — represents a USB device discovered on a specific node.Add a comment on  line R3796Add diff commentMarkdown input:  edit mode selected.WritePreviewAdd a suggestionHeadingBoldItalicQuoteCodeLinkUnordered listNumbered listTask listMentionReferenceMore Formatting tools items 0Saved repliesAdd FilesPaste, drop, or click to add filesCancelCommentStart a review
- [USBDevice](/modules/virtualization/cr.html#usbdevice) (namespaced resource) — represents a USB device available for attachment to virtual machines in a given namespace.

## How it works

USB device passthrough follows a defined lifecycle — from device discovery on a node to attachment to a virtual machine:

1. The DRA driver discovers USB devices on cluster nodes and publishes them to the Kubernetes API as ResourceSlices. The module controller creates [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resources from that data.

1. An administrator assigns a namespace to the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource by setting the `.spec.assignedNamespace` resource field. This makes the device available in that namespace.

1. After the namespace is assigned, the module controller automatically creates a corresponding [USBDevice](/modules/virtualization/cr.html#usbdevice) resource in that namespace.

1. The [USBDevice](/modules/virtualization/cr.html#usbdevice) is attached to a virtual machine by adding it to the `.spec.usbDevices` field of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

## Quick start

The following steps describe the minimal workflow for attaching a USB device to a virtual machine:

1. Connect the USB device to a cluster node.
1. Verify that a [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource has been created:

   ```bash
   d8 k get nodeusbdevice
   ```

1. Assign a namespace to the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) by setting the `.spec.assignedNamespace` resource field.

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

1. Add the device to the `.spec.usbDevices` field of a [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

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

## NodeUSBDevice

[NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource reflects the state of a physical USB device detected on a cluster node. It is a cluster-wide resource that represents a physical USB device on a node.

Example of viewing all discovered USB devices:

```bash
d8 k get nodeusbdevice
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME                 NODE           READY   ASSIGNED   NAMESPACE   AGE
usb-flash-drive      node-1         True    False                  10m
logitech-webcam      node-2         True    True       my-project  15m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

### NodeUSBDevice conditions

The status of a [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource is represented by a set of conditions that describe its availability and assignment state:

- **Ready**: Indicates whether the device is ready to use.
  - `Ready`: Device is ready to use.
  - `NotReady`: Device exists but is not ready.
  - `NotFound`: Device is absent on the host.

- **Assigned**: Indicates whether a namespace is assigned to the device.
  - `Assigned`: Namespace is assigned and [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is created.
  - `Available`: No namespace is assigned for the device.
  - `InProgress`: Device connection to namespace is in progress.

### Assigning a namespace

Before a USB device can be attached to a virtual machine, it must be exposed to a specific namespace. To make a USB device available in a specific namespace, set the `.spec.assignedNamespace` parameter of the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource:

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

## USBDevice

When the related [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) has the `.spec.assignedNamespace` field set, a corresponding [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is created in that namespace. It is a namespaced resource that represents a USB device available for attachment to virtual machines within a given namespace.

Example of viewing USB devices in a namespace:

```bash
d8 k get usbdevice -n my-project
```

Example output:

<!-- markdownlint-disable MD031 -->
```console
NAME               NODE     MANUFACTURER   PRODUCT              SERIAL       ATTACHED   AGE
logitech-webcam    node-2   Logitech       Webcam C920         ABC123456   False      10m
```
{: .nowrap-default }
<!-- markdownlint-enable MD031 -->

### USBDevice attributes

The [USBDevice](/modules/virtualization/cr.html#usbdevice) resource exposes detailed information about the physical USB device. These attributes are available in `.status.attributes`:

- `vendorID`: USB vendor ID (hexadecimal format).
- `productID`: USB product ID (hexadecimal format).
- `bus`: USB bus number.
- `deviceNumber`: USB device number on the bus.
- `serial`: Device serial number.
- `manufacturer`: Device manufacturer name.
- `product`: Device product name.
- `name`: Device name.

### USBDevice conditions

The [USBDevice](/modules/virtualization/cr.html#usbdevice) resource provides status conditions that reflect its readiness and attachment state.

- **Ready**: Indicates whether the device is ready to use.
  - `Ready`: Device is ready to use.
  - `NotReady`: Device exists but is not ready.
  - `NotFound`: Device is absent on the host.

- **Attached**: Indicates whether the device is attached to a virtual machine.
  - `AttachedToVirtualMachine`: Device is attached to a VM.
  - `Available`: Device is available for attachment.
  - `NoFreeUSBIPPort`: Device is requested by a VM but cannot be attached because there are no free USBIP ports on the target node. In this case, `Attached=False`.

## Attaching USB device to VM

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
The USB device is automatically forwarded to the node where the virtual machine is running via the network (USBIP). There is no need to manually place the VM on the same node as the device.
{% endalert %}

{% alert level="warning" %}
During VM migration, the USB device briefly disconnects and reconnects on the new node when the VM switches to it. If migration fails, the device will remain on the original node.
{% endalert %}

## Viewing USB device details

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
Both [USBDevice](/modules/virtualization/cr.html#usbdevice) and [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resources update their status conditions to indicate that the device is no longer present on the host.
{% endalert %}

## Requirements and limitations

USB device passthrough has several operational requirements and limitations that must be considered before use:

- The DRA driver must be installed on nodes where USB devices are to be discovered.
- USB devices are forwarded to the VM node over the network using USBIP. The VM does not need to run on the same node where the device is physically connected. When connecting over the network, the following limitations on the number of devices and hub selection apply:
  - Node can attach at most 16 USB devices: up to 8 on the USB 2.0 hub and up to 8 on the USB 3.0 hub.
  - Hub is determined by the device speed and cannot be changed. A device that operates at USB 2.0 speed cannot be attached to the USB 3.0 hub, and vice versa.
- USB devices support hot-plug — they can be attached to and detached from a running VM without stopping it.
- USB device passthrough requires proper kernel modules on the node.
