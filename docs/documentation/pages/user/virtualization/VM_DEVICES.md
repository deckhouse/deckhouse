---
title: "Device passthrough to a virtual machine"
permalink: en/user/virtualization/vm-devices.html
description: "Attaching passed-through GPU, USB, and PCI devices to a project virtual machine."
search: VM devices, GPU, USB, PCI, device passthrough
---

An administrator provides the project with node devices, and you attach them to your machines. A device is occupied by a single machine and is attached when that machine starts.

## GPU devices

{% alert level="warning" %}
GPU device passthrough is an experimental feature available in commercial DP editions.
{% endalert %}

The virtualization module attaches physical GPU devices to virtual machines using DRA (Dynamic Resource Allocation). A device is requested by a reference to a `GPUClass` in the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

An administrator prepares the `GPUClass` resources, so ask them which classes are available in the cluster.

To request a GPU device, add the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block to the machine specification:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # ... other VM settings ...
  gpus:
    - gpuClassName: nvidia-h100
```

In the `gpuClassName` parameter, specify the name of an existing `GPUClass` resource. To attach several devices, add more elements to the list, their order doesn't matter. A single machine takes no more than 16 devices.

A change to the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block applies only after the virtual machine restarts.

## USB devices

{% alert level="warning" %}
USB device passthrough is available in commercial DP editions.
{% endalert %}

The virtualization module supports USB device passthrough to virtual machines using DRA (Dynamic Resource Allocation). The physical device is connected to a cluster node, and the virtual machine works with it as if the device were plugged into the machine itself.

An administrator connects the device to a node and makes it available to your namespace. After that, a [USBDevice](/modules/virtualization/cr.html#usbdevice) resource appears in the namespace, and you attach it to a virtual machine. If the device you need isn't in the list, contact the administrator.

The administrator takes care of the node and cluster version requirements, so they don't depend on you.

### Project devices (USBDevice)

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

#### USBDevice conditions

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

### Attaching a USB device to a VM

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

## PCI devices

{% alert level="warning" %}
PCI device passthrough is available in commercial DP editions.
{% endalert %}

PCI device passthrough lets you use a physical device of a node in a virtual machine (VM), for example an industrial controller, a hardware security module, a capture card, or an FPGA. The device works in the guest operating system under its own driver, so install that driver in the guest system yourself.

Devices are granted to a project by the administrator, who also handles the requirements for the nodes and the cluster versions.

### Attaching a PCI device to a VM

The devices that the administrator has made available to your project appear in the namespace as [PCIDevice](/modules/virtualization/cr.html#pcidevice) resources. To attach such a device to a machine, follow these steps.

1. Choose a device among the available ones:

   ```bash
   d8 k get pcidevice -n my-project
   ```

   Example output:

   ```console
   NAME                                            NODE     ADDRESS        ATTACHED   AGE
   pci-4f2c0b1e8d9a3c5b7e1f0a2d4c6b8e0f1a3c5d7e    node-1   0000:3b:00.0   False      10m
   ```
   {: .nowrap-default }

   If the list is empty, contact the administrator so that they assign a device to your namespace.

1. Add the device to the [`.spec.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-pcidevices) parameter of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachine
   metadata:
     name: linux-vm
   spec:
     # ... other VM settings ...
     pciDevices:
       - name: pci-4f2c0b1e8d9a3c5b7e1f0a2d4c6b8e0f1a3c5d7e
   EOF
   ```

   Devices are attached when the machine starts, so a change to this parameter requires its restart.

1. Make sure that the device is attached to the machine:

   ```bash
   d8 k get vm linux-vm -o jsonpath='{.status.pciDevices}'
   ```

   The `true` value in the `ready` field means that the device is available, and the `true` value in the `attached` field means that the device is attached to the running machine. Once the machine starts, the device appears in the guest system, for example in the `lspci` output.

To detach a device, remove it from the [`.spec.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-pcidevices) parameter and restart the machine. Until then, the device stays occupied even if the machine is stopped, so you can't attach it to another one.

If the device is removed from the node, the machine doesn't start and stays in the `Pending` phase. In the [`.status.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-status-pcidevices) parameter, such a device stops being ready, but it stays with the machine as long as its specification lists it. The module doesn't stop a running machine in the meantime, including when the device becomes unavailable to the project.

### Limitations

When attaching PCI devices, consider the following limitations:

- A machine with a PCI device starts only on the node of that device and can't be moved by live migration, so it has to be stopped when the node is put into maintenance.
- All PCI devices of a machine have to be on the same node, otherwise the specification is rejected.
- A device is attached to one machine only. If another one already lists it, the module rejects your specification.
- A machine takes no more than eight PCI devices.
- A network card that's passed through works around the cluster network subsystem. It doesn't appear in the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) parameter, and the module neither assigns nor tracks its IP and MAC addresses.
