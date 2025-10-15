---
title: "Requirements"
permalink: en/virtualization-platform/documentation/about/requirements.html
---

{% alert level="warning" %}
The platform components must be deployed on physical servers (bare-metal servers).

Installation on virtual machines is allowed for demonstration purposes only, but nested virtualization must be enabled. If the platform is deployed on virtual machines, technical support will not be provided.
{% endalert %}

## Platform scalability

The platform supports the following configuration:

- Maximum number of nodes: 1000.
- Maximum number of virtual machines: 50000.

## Minimum platform requirements

Depending on the architecture, the following minimum resources are required for the platform to operate correctly:

| Architecture                                                             | Workload placement   | Master node          | Worker node         | System node          | Frontend node       |
|--------------------------------------------------------------------------|----------------------|----------------------|---------------------|----------------------|---------------------|
| Single-node platform<br/>(Single Node / Edge)                            | On a single node     | 3 vCPU<br/>10 GB RAM | —                   | —                    | —                   |
| Multi-node platform<br/>(1 master node + worker nodes)                   | On all nodes         | 6 vCPU<br/>6 GB RAM  | 2 vCPU<br/>4 GB RAM | —                    | —                   |
| Three-master platform<br/>(3 master nodes, High Availability)            | On all nodes         | 6 vCPU<br/>14 GB RAM | —                   | —                    | —                   |
| Platform with dedicated worker nodes<br/>(3 master nodes + worker nodes) | On worker nodes only | 5 vCPU<br/>11 GB RAM | 2 vCPU<br/>5 GB RAM | —                    | —                   |
| Distributed architecture                                                 | On worker nodes only | 4 vCPU<br/>9 GB RAM  | 1 vCPU<br/>2 GB RAM | 4 vCPU<br/>10 GB RAM | 1 vCPU<br/>2 GB RAM |

The choice of platform architecture is described in detail in the [Architecture options](/products/virtualization-platform/documentation/about/architecture-options.html) section.

## Hardware Requirements

### Requirements for the installation machine

The Deckhouse installer runs on this machine. It can be an administrator's laptop or any other computer that is not intended to be added to the cluster. Requirements for this machine:

- OS: Windows 10+, macOS 10.15+, Linux (Ubuntu 18.04+, Fedora 35+);
- Installed Docker Engine or Docker Desktop (instructions for [Ubuntu](https://docs.docker.com/engine/install/ubuntu/), [macOS](https://docs.docker.com/desktop/mac/install/), [Windows](https://docs.docker.com/desktop/windows/install/));
- HTTPS access to the container image registry `registry.deckhouse.ru`;
- SSH key-based access to the node that will become the cluster **master node**;
- SSH key-based access to the node that will become the cluster **worker node** (if the cluster will contain more than one master node).

### General requirements for physical servers (bare-metal)

All cluster nodes must meet the following baseline hardware requirements:

- **CPU**:
  - x86_64 architecture;
  - Intel-VT (VMX) or AMD-V (SVM) support.
- **Compatibility**:
  - The platform has no additional restrictions and can run on any server hardware supported by the selected operating system.
- **Resources**:
  - CPU, RAM, and disk must meet the selected cluster architecture (see [minimum requirements](#minimum-platform-requirements));
  - Fast disk (≥400 IOPS), at least 60 GB capacity;
  - Additional disks may be required when using SDS.
- **Operating system** — [from the supported list](#supported-os-for-platform-nodes), Linux kernel version `5.8` or newer.
- **Software**:
  - Installed `cloud-init` and `cloud-utils` packages (package names may vary depending on the distribution).
- **Networking**:
  - HTTPS access to `registry.deckhouse.ru` and OS package repositories;
  - SSH access from the installation machine on port `22/TCP` (see details in [requirements for the installation machine](#requirements-for-the-installation-machine));
  - Unique hostname across all cluster nodes.

{% alert level="warning" %}
The container runtime will be installed automatically, so the `containerd` and/or `docker` packages must not be preinstalled.
{% endalert %}

#### Additional requirements for master nodes

Master nodes host the cluster control plane components. Minimum resource requirements for master nodes are specified in the [minimum requirements](#minimum-platform-requirements) table.

#### Additional requirements for worker nodes

Worker nodes host virtual machines. Resource requirements depend on the number and size of the planned VMs (see details in the [minimum platform requirements](#minimum-platform-requirements)). If SDS is used, additional dedicated disk space may be required for storage.

### Storage hardware requirements

Depending on the selected storage type, additional resources may be required. For details, see [Storage Management](/products/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html).

## Supported OS for platform nodes

| Linux distribution | Supported versions  |
| ------------------ | ------------------- |
| CentOS             | 7, 8, 9             |
| Debian             | 10, 11, 12          |
| Ubuntu             | 20.04, 22.04, 24.04 |

{% alert level="warning" %}
Ensuring stable operation of live migration mechanisms requires the use of an identical version of the Linux kernel on all cluster nodes.

This is because differences in kernel versions can lead to incompatible interfaces, system calls, and resource handling, which can disrupt the virtual machine migration process.
{% endalert %}

## Supported guest operating systems

The virtualization platform supports operating systems running on `x86` and `x86_64` architectures as guest operating systems. For correct operation in paravirtualization mode, `VirtIO` drivers must be installed to ensure efficient interaction between the virtual machine and the hypervisor.

Successful startup of the operating system is determined by the following criteria:

- Correct installation and booting of the OS;
- Uninterrupted operation of key components such as networking and storage;
- No crashes or errors during operation.

For Linux family operating systems, it is recommended to use guest OS images with `cloud-init` support, which allows initializing virtual machines after their creation.

For Windows family operating systems, the platform supports initialization with [autounattend](https://learn.microsoft.com/ru-ru/windows-hardware/manufacture/desktop/windows-setup-automation-overview) installation.

## Supported virtual machine configurations

- Maximum number of cores supported: `248`.
- Maximum amount of RAM: `1024 GB`.
- Maximum number of block devices to be attached: `16`.

## Supported storage systems

Virtual machines use PersistentVolume resources. To manage these resources and allocate disk space within the cluster, one or more supported storage systems must be installed:

| Storage System            | Disk Location             |
| ------------------------- | ------------------------- |
| sds-local-volume          | Local                     |
| sds-replicated-volume     | Replicas on cluster nodes |
| Ceph Cluster              | External storage          |
| NFS (Network File System) | External storage          |
| TATLIN.UNIFIED (Yadro)    | External storage          |
| Huawei Dorado             | External storage          |
| HPE 3par                  | External storage          |
