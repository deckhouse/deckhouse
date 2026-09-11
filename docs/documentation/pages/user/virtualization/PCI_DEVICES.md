---
title: "PCI devices in a virtual machine"
permalink: en/user/virtualization/pci-devices.html
description: "Attaching project PCI devices to a virtual machine and the limitations of such a machine."
search: PCI in a VM, PCI passthrough, PCIDevice, migration limitations
---

{% alert level="warning" %}
PCI device passthrough is available in commercial DP editions.
{% endalert %}

PCI device passthrough lets you use a physical device of a node in a virtual machine (VM), for example an industrial controller, a hardware security module, a capture card, or an FPGA. The device works in the guest operating system under its own driver, so install that driver in the guest system yourself.

Devices are granted to a project by the administrator, who also handles the requirements for the nodes and the cluster versions.

## Attaching a PCI device to a VM

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

## Limitations

When attaching PCI devices, consider the following limitations:

- A machine with a PCI device starts only on the node of that device and can't be moved by live migration, so it has to be stopped when the node is put into maintenance.
- All PCI devices of a machine have to be on the same node, otherwise the specification is rejected.
- A device is attached to one machine only. If another one already lists it, the module rejects your specification.
- A machine takes no more than eight PCI devices.
- A network card that's passed through works around the cluster network subsystem. It doesn't appear in the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) parameter, and the module neither assigns nor tracks its IP and MAC addresses.
