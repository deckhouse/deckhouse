---
title: "Virtualization"
permalink: en/admin/configuration/virtualization/
description: "Managing virtual machines in Deckhouse Platform: configuring the virtualization module, images, virtual machine classes, and device passthrough."
search: virtualization, virtual machines, virtualization module
---

The [`virtualization`](/modules/virtualization/) module runs virtual machines (VMs) next to container workloads in the same cluster and manages them declaratively, through Kubernetes resources. For the module internals, refer to the [Virtualization subsystem](../../../architecture/virtualization/) section; for the full list of resources and their parameters, refer to the [module reference](/modules/virtualization/cr.html).

A cluster administrator prepares the platform for running virtual machines and provides projects with the resources their owners use.

**Installing and configuring the module:**

- [Virtualization requirements and limits](requirements.html): Hardware, software, guest operating systems, and supported storage.
- [Installing and updating the virtualization module](install.html): Enabling the module, placing its components, and switching release channels.
- [Virtualization module parameters](settings.html): The ModuleConfig resource, the configuration version, and Ingress settings.

**Storing images and disks:**

- [Virtual machine image storage](image-storage.html): The DVCR volume and cleanup of stale data.
- [Storage for virtual machine disks and images](storage-classes.html): Which storage classes are available to projects.
- [Cluster images of virtual machines](cluster-images.html): Cluster-wide images that disks are created from.

**Virtual machine classes:**

- [Virtual machine classes](vm-classes.html): The VirtualMachineClass resource and the default class.
- [Virtual processor of virtual machines](vm-classes-cpu.html): The processor type and instruction set the guest system sees.
- [Placement of virtual machines across nodes](vm-classes-placement.html): Which nodes machines of the class run on.
- [Virtual machine sizing policy](vm-classes-sizing.html): Allowed combinations of cores, memory, and core fraction.
- [CPU oversubscription for virtual machines](cpu-oversubscription.html): The share of a physical core that a machine gets.

**Device passthrough:** [USB devices](usb-devices.html), [GPU devices](gpu-devices.html), and [PCI devices](pci-devices.html).

Related topics are covered in other chapters:

- [Virtual machine networking](../network/vm-network.html): Subnets that machines get addresses from.
- [Maintenance of nodes running virtual machines](../platform-scaling/node/vm-node-maintenance.html): Taking a node out for maintenance.
- [Virtual machine fault tolerance and balancing](../high-reliability-and-availability/vm-reliability.html): Rebalancing and ColdStandby.
- [Virtualization event audit](../security/events/virtualization-audit.html): The log of actions on module resources.

The [Virtual machines](../../../user/virtualization/) section is addressed to a project owner.
