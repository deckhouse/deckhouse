---
title: "USB devices in a virtual machine"
permalink: en/user/virtualization/usb-devices.html
description: "Attaching project USB devices to a virtual machine, including without stopping it."
search: USB in a VM, USB passthrough, USBDevice, attaching a device
---

{% alert level="warning" %}
USB device passthrough is available in commercial DP editions.
{% endalert %}

The virtualization module supports USB device passthrough to virtual machines using DRA (Dynamic Resource Allocation). The physical device is connected to a cluster node, and the virtual machine works with it as if the device were plugged into the machine itself.

An administrator connects the device to a node and makes it available to your namespace. After that, a [USBDevice](/modules/virtualization/cr.html#usbdevice) resource appears in the namespace, and you attach it to a virtual machine. If the device you need isn't in the list, contact the administrator.

The administrator takes care of the node and cluster version requirements, so they don't depend on you.

## Project devices (USBDevice)

[USBDevice](/modules/virtualization/cr.html#usbdevice) is a namespaced resource that represents a USB device available for attaching to virtual machines in a given namespace. It appears automatically after an administrator assigns the device to the namespace.

An example of viewing the USB devices in a namespace:

```bash
d8 k get usbdevice -n my-project
```

Example output:

```console
NAME              NODE     MANUFACTURER   PRODUCT       ATTACHED   AGE
logitech-webcam   node-2   Logitech       Webcam C920   False      10m
```
{: .nowrap-default }

The resource keeps the vendor and product identifiers, the bus, the device number, the serial number, the speed, and the rest of the device details in the [`.status.attributes`](/modules/virtualization/cr.html#usbdevice-v1alpha2-status-attributes) block.

### USBDevice conditions

Two conditions in the [`.status.conditions`](/modules/virtualization/cr.html#usbdevice-v1alpha2-status-conditions) block describe the device state.

The `Ready` condition shows whether the device is ready for use, and takes one of the following reasons:

- `Ready`: The device is ready for use.
- `NotReady`: The device exists but isn't ready.
- `NotFound`: The device is absent from the node.

The `Attached` condition shows whether the device is attached to a virtual machine:

- `AttachedToVirtualMachine`: The device is attached to a VM.
- `Available`: The device is free and can be attached.
- `DetachedForMigration`: The device is detached for the duration of a VM migration and is attached again on the target node.
- `NoFreeUSBIPPort`: The device is requested by a virtual machine, but the target node has no free USBIP ports left, so the condition has the `False` status.

## Attaching a USB device to a VM

A device is attached to and detached from a machine without stopping it.

{% tabs usb-attach %}

{% tab "Using the CLI" %}

Once a [USBDevice](/modules/virtualization/cr.html#usbdevice) resource appears in the namespace, you can attach it to a virtual machine. To do this, add the device to the [`.spec.usbDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-usbdevices) parameter of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource:

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

After the VM is created or updated, the USB device is attached to the specified virtual machine.

> The USB device is automatically passed through over the network (USBIP) to the node where the virtual machine runs. You don't have to place the VM manually on the same node as the device.

> **Important:** During a VM migration, the USB device briefly disconnects and reconnects on the new node at the moment the VM switches over. If the migration fails, the device stays on the old node.

You can attach a USB device to a running VM and detach it without stopping the machine.

Infrastructure requirements, USBIP port limits, and device discovery on nodes are described in [USB devices in virtual machines](../../admin/configuration/virtualization/usb-devices.html).

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the VM you need from the list and click its name.
1. On the **Configuration** tab, scroll down to the **USB devices** section and click **Add**.
1. In the **Attach USB device** window that opens, select the device in the **Select USB device** field and click **Add**.
1. Click the **Save** button that appears.

The USB devices available in the project are shown in **Virtualization** → **USB devices**: the resource name, status, manufacturer, product, serial number, node, bus, and device number.

{% endtab %}

{% endtabs %}
