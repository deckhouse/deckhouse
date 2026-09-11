---
title: "PCI devices in virtual machines"
permalink: en/admin/configuration/virtualization/pci-devices.html
description: "PCI device passthrough to virtual machines: node requirements, the NodePCIDevice and PCIDevice resources, assigning a device to a namespace, limitations."
search: PCI devices, PCI passthrough, NodePCIDevice, PCIDevice, vfio-pci, IOMMU
---

{% alert level="warning" %}
PCI device passthrough is available in commercial DP editions.
{% endalert %}

PCI device passthrough lets you attach a physical device of a node to a virtual machine (VM), for example an industrial controller, a hardware security module, a capture card, an FPGA, or an entire network card. In the guest operating system, such a device works under its own driver, so a VM can use the hardware the module doesn't support directly.

A device reaches a machine in two steps. First you assign it to a project namespace, and then the project owner lists the device in the specification of their machine. The device is granted for exclusive use, so it's available in one namespace and to one machine only.

The module switches the device drivers itself, and you don't need to configure them on the node manually. When a machine starts, the module unbinds the device from the regular kernel driver and binds it to the `vfio-pci` driver, and once the machine is stopped, it returns the device to the regular driver. While the machine is running, the node doesn't use the device.

## Node requirements

PCI device passthrough is handled by the `virtualization-dra-pci` system component. It runs only on nodes with containerd version 2 and hardware I/O virtualization enabled. The module checks the nodes itself and assigns the `virtualization.deckhouse.io/vfio=true` label to the suitable ones.

To enable hardware I/O virtualization, turn on `VT-d` on Intel or `AMD-Vi` on AMD in the node BIOS and add the `intel_iommu=on` or `amd_iommu=on` kernel parameter.

To see which nodes are ready for PCI device passthrough, run the following command:

```bash
d8 k get nodes -l virtualization.deckhouse.io/vfio=true,virtualization.deckhouse.io/containerd-version=v2
```

Example output:

```console
NAME     STATUS   ROLES    AGE   VERSION
node-1   Ready    worker   10d   v1.34.1
```

A node missing from the output has not been assigned the label. Check the `/sys/kernel/iommu_groups` directory on it. An empty directory means that hardware I/O virtualization is disabled. Enable it and reboot the node, and the label is assigned automatically within a few minutes.

To verify that the component is actually running on these nodes, run the following command:

```bash
d8 k -n d8-virtualization get pods -l app=virtualization-dra-pci -o wide
```

## Assigning a namespace to a PCI device

The module detects the devices on the suitable nodes and creates a [NodePCIDevice](/modules/virtualization/cr.html#nodepcidevice) resource for each of them. The component scans the PCI bus at startup and then every five minutes, so a newly installed device appears in the list within a few minutes.

To make a device available to a project, follow these steps.

1. Find the device among the detected ones:

   ```bash
   d8 k get nodepcidevice
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                                            NODE     ADDRESS        READY   ASSIGNED   ATTACHED   NAMESPACE    AGE
   pci-4f2c0b1e8d9a3c5b7e1f0a2d4c6b8e0f1a3c5d7e    node-1   0000:3b:00.0   True    False      False                   10m
   pci-9a1b3c5d7e9f0a2b4c6d8e0f1a3b5c7d9e1f0a2b    node-2   0000:65:00.0   True    True       False      my-project   15m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   The resource name is a hash of the device parameters and the node name, so look up the device you need by the `NODE` and `ADDRESS` columns. To verify the choice, use the [`.status.attributes`](/modules/virtualization/cr.html#nodepcidevice-v1alpha2-status-attributes) block, which holds the PCI bus address and the vendor and model identifiers. They let you find the same device in the `lspci -nn` output on the node.

1. Assign a namespace with the [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodepcidevice-v1alpha2-spec-assignednamespace) parameter:

   ```bash
   d8 k apply -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: NodePCIDevice
   metadata:
     name: pci-4f2c0b1e8d9a3c5b7e1f0a2d4c6b8e0f1a3c5d7e
   spec:
     assignedNamespace: my-project
   EOF
   ```

1. Make sure that a [PCIDevice](/modules/virtualization/cr.html#pcidevice) resource has appeared in the namespace:

   ```bash
   d8 k get pcidevice -n my-project
   ```

After that, the project owner attaches the device to their machine by specifying the [PCIDevice](/modules/virtualization/cr.html#pcidevice) resource name in the [`.spec.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-pcidevices) parameter of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

To make the device unavailable to the project, clear the [`.spec.assignedNamespace`](/modules/virtualization/cr.html#nodepcidevice-v1alpha2-spec-assignednamespace) parameter or set a different namespace. The [PCIDevice](/modules/virtualization/cr.html#pcidevice) resource in the previous namespace is deleted.

While the device is listed in a machine specification, the [PCIDevice](/modules/virtualization/cr.html#pcidevice) resource isn't deleted and the machine keeps running. Ask the project owner to remove the device from the specification. If you set a different namespace instead of clearing the parameter, the resource appears in it right away, but the machine of the new project stays in the `Pending` phase while the device is occupied by the previous machine.

## Requirements and limitations for PCI devices

When planning PCI device passthrough, consider the following requirements and limitations:

- Passthrough works in a cluster with [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) 1.34 or higher and containerd version 2 on the nodes, and the `DRAResourceClaimDeviceStatus`, `DRADeviceBindingConditions`, and `DRAConsumableCapacity` feature gates have to be enabled in kube-apiserver.
- The module doesn't detect every device of a node, because the hardware the node itself depends on stays at its disposal. The following devices never appear in the list:
  - Display adapters, which are handled by the GPU module.
  - Devices integrated into the chipset, bridges, memory controllers, and system peripherals.
  - Network controllers with active interfaces.
  - Storage controllers whose disks are used by the node.
  - Devices that share an IOMMU group with any of the above, because a group goes to the VM as a whole.
- A VM starts only on the node that holds the device attached to it, so all its PCI devices have to be on the same node, otherwise the specification is rejected.
- A VM with a PCI device can't be moved by live migration, and the `Migratable` condition gets the `VirtualMachineHostDevicesNotMigratable` reason, so such machines have to be stopped before the node is put into maintenance.
- Devices are attached when the VM starts, so a change to the [`.spec.pciDevices`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-pcidevices) parameter requires its restart.
- A single device is attached to one VM only, and a VM takes no more than eight devices.
- A network card that's passed through works around the cluster network subsystem. It doesn't appear in the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) parameter, and the module neither assigns nor tracks its IP and MAC addresses.
