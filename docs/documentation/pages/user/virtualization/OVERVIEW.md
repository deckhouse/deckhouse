---
title: "Virtual machines"
permalink: en/user/virtualization/
description: "Virtual machines in a Deckhouse Platform project: images, disks, working with a machine, snapshots, cloning, pools, and device passthrough."
search: virtual machines, VM, virtualization, images, disks
---

In a Deckhouse Platform (DP) project, virtual machines (VMs) run next to container workloads. A machine, its disks, and its images are described by Kubernetes resources, so you create, change, and delete them the same way as the other objects of the project.

Cluster-wide resources are prepared by an administrator: virtual machine classes, cluster images, and the storage and subnets available to the project. If the project lacks the class or image you need, contact the administrator.

To go all the way from creating an image to logging in to the guest system over SSH, and to clean up the created resources afterwards, refer to [Quick start](quickstart.html).

## Images and disks

Machine data is stored on a disk, and the data source for that disk is an image. Which sources are supported and how to store an image so that disks are created from it faster is covered in [Virtual machine images](images.html).

Working with disks is covered in three sections: [Virtual machine disks](disks.html) for creating a disk and choosing its storage, [Resizing and migrating virtual machine disks](disk-operations.html) for expanding a disk and moving it to another storage, and [Exporting virtual machine disk data](data-export.html) for downloading disk contents outside the cluster.

## Working with a virtual machine

The minimum set of parameters and the machine start are covered in [Creating a virtual machine](vm-create.html). Each group of parameters is then covered separately:

- [CPU and memory of a virtual machine](vm-resources.html): The number of cores, the core fraction, and the memory size.
- [Guest OS and bootloader of a virtual machine](vm-os-and-boot.html): The operating system type and the boot mode.
- [Provisioning and the guest OS agent](vm-provisioning.html): cloud-init, Sysprep, and `qemu-guest-agent`.
- [Placement of virtual machines across nodes](vm-placement.html): Node selectors and affinity rules.
- [Attaching disks and images to a virtual machine](vm-block-devices.html): Devices in the machine spec and as a separate resource.

You can connect to a running machine over SSH, through the serial console, or over VNC, as covered in [Connecting to a virtual machine](vm-access.html). Which configuration changes apply right away and which require a restart is covered in [Changing the configuration of a virtual machine](vm-configuration.html). To move a running machine to another node, refer to [Live migration of virtual machines](vm-migration.html); to collect the state of a machine and its related resources for a support request, refer to [Collecting debug information about a virtual machine](vm-debug.html).

## Snapshots, cloning, and pools

The state of a disk or of a whole machine is saved by a snapshot, which a new disk is later created from or the machine itself is restored from. This is covered in [Snapshots of disks and virtual machines](snapshots.html). To get a copy of a machine along with its disks without stopping it, refer to [Cloning virtual machines](vm-cloning.html); to keep a given number of identical machines and scale them, refer to [Virtual machine pools](vm-pools.html).

## Networking

A machine gets an address in the main cluster network automatically, but you can choose the address in advance and keep it reserved for the project, as covered in [IP addresses of virtual machines](../network/virtualization/vm-ip-addresses.html). To open a machine application to other cluster workloads or to the outside, refer to [Access to applications on a virtual machine](../network/virtualization/vm-publishing.html). Besides the main network, a machine can be connected to project and cluster networks, which is covered in [Additional network interfaces of virtual machines](../network/virtualization/vm-additional-interfaces.html).

## Device passthrough

If an administrator has prepared devices on the nodes, you can give them to a machine: [GPU devices](gpu-devices.html), [USB devices](usb-devices.html), and [PCI devices](pci-devices.html).

The full list of resources and their parameters is provided in the [reference](/modules/virtualization/cr.html). DP configuration for virtualization is covered in the [Virtualization](../../admin/configuration/virtualization/) section.
