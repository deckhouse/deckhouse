---
title: "USB devices in virtual machines"
permalink: en/admin/configuration/virtualization/usb-devices.html
description: "USB device passthrough to virtual machines: preparing nodes, the NodeUSBDevice resource, assigning a device to a project, requirements and limitations."
search: USB devices, USB passthrough, NodeUSBDevice, usbip
---

{% alert level="warning" %}
USB device passthrough is available in commercial DP editions.
{% endalert %}

USB device passthrough to virtual machines (VMs) is handled by the `virtualization-dra` system component, which needs three kernel modules on the node:

- `usbip_core`
- `usbip_host`
- `vhci_hcd`

The module loads them on the nodes itself. A node where all three modules are available gets the `virtualization.deckhouse.io/usbip=true` label, and the `virtualization-dra` component runs only on such nodes. If the kernel modules stop being available, the label is removed and the component is deleted from the node.

To see which nodes are ready for USB device passthrough, run the following command:

```bash
d8 k get nodes -l virtualization.deckhouse.io/usbip=true
```

Example output:

```console
NAME     STATUS   ROLES    AGE   VERSION
node-1   Ready    worker   10d   v1.34.1
```

To verify that the component is actually running on these nodes, run the following command:

```bash
d8 k -n d8-virtualization get pods -l app=virtualization-dra -o wide
```

A node missing from the output failed to load the kernel modules, and USB devices on that node aren't detected. Install the kernel modules yourself from your operating system package, or build them for the kernel in use. The module detects them on its own and assigns the label to the node within a few minutes.

## Path of a USB device from a node to a VM

A USB device travels from the node to a virtual machine in four steps:

1. The DRA driver detects USB devices on the nodes and publishes information about them to the Kubernetes API as a [ResourceSlice](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/). The module controller creates [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resources from this data.

1. The administrator assigns a namespace to the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource by setting the [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodeusbdevice-v1alpha2-spec-assignednamespace) parameter. This makes the device available in that namespace.

1. Once the namespace is assigned, the module controller creates a [USBDevice](/modules/virtualization/cr.html#usbdevice) resource in it.

1. The project owner attaches the [USBDevice](/modules/virtualization/cr.html#usbdevice) device to a virtual machine by adding it to the [`.spec.usbDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-usbdevices) parameter of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

## Discovered devices (NodeUSBDevice)

The [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource describes a physical USB device detected on a node. The resource exists at the cluster level, so you see all detected devices in a single list:

```bash
d8 k get nodeusbdevice
```

Example output:

```console
NAME              NODE     READY   ASSIGNED   ATTACHED   NAMESPACE    AGE
usb-flash-drive   node-1   True    False      False                   10m
logitech-webcam   node-2   True    True       True       my-project   15m
```
{: .nowrap-default }

The conditions in the [`.status.conditions`](/modules/virtualization/cr.html#nodeusbdevice-v1alpha2-status-conditions) block reflect the readiness of the device and its state. The `Ready` and `Attached` conditions match the [USBDevice conditions](../../../user/virtualization/usb-devices.html#usbdevice-conditions), and the `Assigned` condition shows whether a namespace is assigned to the device:

- `Available`: No namespace is assigned.
- `InProgress`: A namespace is assigned and the [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is being created.
- `Assigned`: The [USBDevice](/modules/virtualization/cr.html#usbdevice) resource is created and the device is available in the namespace.

### Assigning a namespace to a USB device

Until a namespace is assigned to a device, the project owner doesn't see it. To make the device available in a project, follow these steps.

1. Connect the USB device to a node that is ready for passthrough and wait for a [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource to appear.

1. Assign the namespace with the [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodeusbdevice-v1alpha2-spec-assignednamespace) parameter:

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

1. Verify that a [USBDevice](/modules/virtualization/cr.html#usbdevice) resource appears in the namespace:

   ```bash
   d8 k get usbdevice -n my-project
   ```

After that, the project owner attaches the device to a virtual machine.

## Viewing USB device details

Full details about a device and its current state are available in the resource status.

{% tabs usb-view %}

{% tab "Using the CLI" %}

The device identifiers, its location, and the current conditions are stored in the resource status:

```bash
d8 k get nodeusbdevice <DEVICE_NAME> -o yaml
```

Where `<DEVICE_NAME>` is the name of the [NodeUSBDevice](/modules/virtualization/cr.html#nodeusbdevice) resource.

To get only the device attributes, query the fields you need directly:

```bash
d8 k get nodeusbdevice <DEVICE_NAME> \
  -o jsonpath='{.status.attributes.manufacturer}{" "}{.status.attributes.product}{" ("}{.status.attributes.vendorID}{":"}{.status.attributes.productID}{")\n"}'
```

Example output:

```console
Logitech Webcam C920 (046d:082d)
```

> When a device is physically disconnected from the node, the `Attached` condition gets the `False` value, and the `Ready` condition gets the `NotFound` reason. The same is reflected in the status of the [USBDevice](/modules/virtualization/cr.html#usbdevice) resource in the project namespace.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Virtualization** → **Node USB devices**.
1. Review the list, which shows the device status, manufacturer, product, serial number, node, bus, device number, and assigned namespace.

{% endtab %}

{% endtabs %}

The project owner sees the devices assigned to their project in the **Virtualization** → **USB devices** section of that project.

## Requirements and limitations

When planning USB device passthrough, consider the following requirements and limitations:

- A node where USB devices must be detected has to carry the `virtualization.deckhouse.io/usbip=true` label and run containerd version 2, otherwise the `virtualization-dra` component doesn't start there.
- A device is passed to a virtual machine over the network using USBIP, so the VM can run on a node other than the one the device is physically connected to.
- Only a device that reports the USB 2.0 speed (480 Mbps) or a USB 3.x speed (5 Gbps and higher) can be passed through. The module doesn't let you attach a slower device to a VM, for example a mouse or a keyboard at 1.5 or 12 Mbps.
- A node connects no more than 16 devices, 8 per USB 2.0 hub and 8 per USB 3.0 hub.
- The hub is selected by the device speed and can't be changed manually. A USB 2.0 device doesn't connect to a USB 3.0 hub, and vice versa.
- A device can be attached to a running VM and detached from it without stopping the VM.
