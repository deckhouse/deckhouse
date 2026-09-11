---
title: "Virtualization"
permalink: en/admin/configuration/virtualization/
description: "Virtualization in Deckhouse Platform: preparing a cluster for running virtual machines, image storage, virtual machine classes, and device passthrough."
search: virtualization, virtual machines, virtual machine classes, device passthrough
---

Deckhouse Platform (DP) runs virtual machines (VMs) in the same cluster as container workloads and manages them declaratively, through Kubernetes resources. Machines run on the same nodes, networks, and storage as the other workloads, so an administrator configures them with the usual DP tools.

DP lets you do the following:

- Prepare cluster nodes for running virtual machines.
- Store cluster-wide images and provide projects with storage for disks.
- Define the machine parameters that project owners can use.
- Pass node devices through to machines.

## Installation and configuration

Virtualization has its own node requirements: hardware support, kernel and software versions, supported guest operating systems and storage. All of them are listed in [Virtualization requirements and limits](requirements.html).

Virtualization is enabled as a DP module, so preparation comes down to installing and configuring it. How to enable virtualization, where to place its components, and how to switch to another release channel is covered in [Installing and updating the virtualization module](install.html), and the parameters set in the ModuleConfig resource are covered in [Virtualization module parameters](settings.html).

## Storing images and disks

Machine images and disks take up space in the cluster, and an administrator configures how they are stored.

DP keeps uploaded images in its own DVCR storage. Its size, storage class, and the schedule for cleaning up stale data are covered in [Virtual machine image storage](image-storage.html). The storage classes that projects create disks and images from are listed in [Storage for virtual machine disks and images](storage-classes.html).

Cluster-wide images are prepared by an administrator: project owners create disks from them without uploading the same data again. How to create such images and in what form to store them is covered in [Cluster images of virtual machines](cluster-images.html).

## Virtual machine classes

A class defines which processor the guest system sees, which nodes a machine runs on, and which combinations of cores and memory are available to it. A project owner picks a class out of the ones prepared by an administrator, so classes are how DP limits machine parameters in the cluster.

The VirtualMachineClass resource and the purpose of the default class are covered in [Virtual machine classes](vm-classes.html). Each group of settings then has a section of its own: [Virtual processor of virtual machines](vm-classes-cpu.html), [Placement of virtual machines across nodes](vm-classes-placement.html), and [Virtual machine sizing policy](vm-classes-sizing.html). [CPU oversubscription for virtual machines](cpu-oversubscription.html), the share of a physical core that a machine gets guaranteed, is covered separately.

## Device passthrough

A machine can be given a node device: a [USB device](usb-devices.html), a [GPU](gpu-devices.html), or a [PCI device](pci-devices.html). Each type has its own node requirements and its own preparation order, so each has a section of its own.

## Operating a cluster with virtual machines

Running machines also affect other areas of DP configuration:

- [Virtual machine networking](../network/vm-network.html): Subnets that machines get addresses from.
- [Maintenance of nodes running virtual machines](../platform-scaling/node/vm-node-maintenance.html): Taking a node out for maintenance without losing machines.
- [Virtual machine fault tolerance and balancing](../high-reliability-and-availability/vm-reliability.html): Rebalancing and ColdStandby.
- [Virtualization event audit](../security/events/virtualization-audit.html): The log of actions on virtualization resources.

For how virtualization works internally, refer to the [Virtualization subsystem](../../../architecture/virtualization/) section; for the full list of resources and their parameters, refer to the [reference](/modules/virtualization/cr.html). The [Virtual machines](../../../user/virtualization/) section is addressed to a project owner.
