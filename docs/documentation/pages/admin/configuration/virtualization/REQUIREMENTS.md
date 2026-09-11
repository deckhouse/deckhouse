---
title: "Virtualization requirements and limits"
permalink: en/admin/configuration/virtualization/requirements.html
description: "Hardware and software requirements, scaling limits, supported guest operating systems, and storage for virtualization."
search: virtualization requirements, scaling limits, guest OS, supported storage
---

{% alert level="warning" %}
Module components must be deployed on physical servers (bare-metal).

Installation on virtual machines is allowed for demonstration purposes only, but nested virtualization must be enabled. If the module is deployed on virtual machines, technical support is not provided.
{% endalert %}

## Scaling limits

The module is designed for a cluster of this size:

- up to `1000` nodes;
- up to `50000` virtual machines.

The module has no additional restrictions and is compatible with any hardware supported by the operating systems on which it can be installed.

## Hardware and software requirements

Hardware requirements for the virtualization module match the requirements for the [Deckhouse Platform](/products/kubernetes-platform/guides/production.html#resource-requirements) (DP), with an additional requirement: CPU virtualization support on the hosts where virtual machines will be launched.

### Additional requirements for virtualization support

On all cluster nodes where virtual machines are planned to be launched, hardware virtualization support must be provided:

- CPU: Support for Intel-VT (VMX) or AMD-V (SVM) instructions.
- BIOS/UEFI: Hardware virtualization support enabled in the BIOS/UEFI settings.

{% alert level="warning" %}
Ensuring the stable operation of live migration mechanisms requires using the same Linux kernel version on all cluster nodes.

Differences between kernel versions can lead to incompatible interfaces, system calls, and resource handling, which can disrupt the virtual machine migration process.
{% endalert %}

It is recommended to use Linux kernels with up-to-date security updates provided by the maintainers of your chosen distribution. Such updates are not a prerequisite for virtualization to function, but they reduce security risks for the cluster as a whole.

{% alert level="info" %}
On Astra Linux nodes, **Astra Linux platform version 1.8.3 or higher** is required for virtualization to work correctly (earlier versions contain a bug that interferes with virtualization).
{% endalert %}

## Supported guest operating systems

The virtualization platform supports operating systems running on `x86` and `x86_64` architectures as guest operating systems. For correct operation in paravirtualization mode, `VirtIO` drivers must be installed to ensure efficient interaction between the virtual machine and the hypervisor.

Successful startup of the operating system is determined by the following criteria:

- Correct installation and booting of the OS.
- Uninterrupted operation of key components such as networking and storage.
- No crashes or errors during operation.

For Linux family operating systems, it is recommended to use guest OS images with `cloud-init` support, which allows initializing virtual machines after their creation.

For Windows family operating systems, the platform supports initialization with [autounattend](https://learn.microsoft.com/en-us/windows-hardware/manufacture/desktop/windows-setup-automation-overview) installation.

## Virtual machine configuration limits

- Maximum number of cores supported: `248`.
- Maximum amount of RAM: `1024 GB`.
- The maximum number of block devices to be attached: `16`.

## Supported storage systems

Virtual machine disks are created using PersistentVolume resources. To manage these resources and allocate disk space in the cluster, one or more supported storage systems must be deployed:

| Storage System            | Disk Location             |
| ------------------------- | ------------------------- |
| sds-local-volume          | Local                     |
| sds-replicated-volume     | Replicas on cluster nodes |
| Ceph Cluster              | External storage          |
| NFS (Network File System) | External storage          |
| TATLIN.UNIFIED (Yadro)    | External storage          |
| Huawei Dorado             | External storage          |
| HPE 3par                  | External storage          |
