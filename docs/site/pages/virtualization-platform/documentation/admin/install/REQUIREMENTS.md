---
title: "Requirements"
permalink: en/virtualization-platform/documentation/admin/install/requirements.html
---

> **Warning.** The platform components must be deployed on physical servers (bare-metal servers).
>
> Installation on virtual machines is allowed for demonstration purposes only, but nested virtualization must be enabled. If the platform is deployed on virtual machines, technical support will not be provided.

## Platform scalability

The platform supports the following configuration:

- Maximum number of nodes: 1000.
- Maximum number of virtual machines: 50000.

The platform has no other restrictions and is compatible with any hardware that is supported by [operating systems](#supported-os-for-platform-nodes) on which it can be installed.

## Hardware Requirements

1. A dedicated **machine for installation**.

   This machine will run the Deckhouse installer. For example, it can be an administrator's laptop or any other computer that is not intended to be added to the cluster. Requirements for this machine:

   - OS: Windows 10+, macOS 10.15+, Linux (Ubuntu 18.04+, Fedora 35+);
   - Installed Docker Engine or Docker Desktop (instructions for [Ubuntu](https://docs.docker.com/engine/install/ubuntu/), [macOS](https://docs.docker.com/desktop/mac/install/), [Windows](https://docs.docker.com/desktop/windows/install/));
   - HTTPS access to the container image registry at `registry.deckhouse.io`;
   - SSH key-based access to the node that will serve as the **master node** of the future cluster;
   - SSH key-based access to the node that will serve as the **worker node** of the future cluster (if the cluster will consist of more than one master node).

1. **Server for the master node**

   There can be multiple servers running the cluster’s control plane components, but only one server is required at installation time. The others can be added later via node management mechanisms.

   Requirements for a physical bare-metal server:

   - Resources:
     - CPU:
       - x86_64 architecture;
       - Support for Intel-VT (VMX) or AMD-V (SVM) instructions;
       - At least 4 cores.
     - RAM: At least 8 GB.
     - Disk space:
       - At least 60 GB;
       - High-speed disk (400+ IOPS).
   - OS [from the list of supported ones](#supported-os-for-platform-nodes):
     - Linux kernel version `5.7` or newer.
   - **Unique hostname** across all servers in the future cluster;
   - Network access:
     - HTTPS access to the container image registry at `registry.deckhouse.io`;
     - Access to the package repositories of the chosen OS;
     - SSH key-based access from the **installation machine** (see p.1);
     - Network access from the **installation machine** (see p.1) on port `22322/TCP`.
   - Required software:
     - The `cloud-utils` and `cloud-init` packages must be installed (package names may vary depending on the chosen OS).
   > **Warning.** The container runtime will be installed automatically, so do not pre-install any `containerd` or `docker` packages.

1. **Servers for worker nodes**

   These nodes will run virtual machines, so the servers must have enough resources to handle the planned number of VMs. Additional disks may be required if you deploy a software-defined storage solution.

   Requirements for a physical bare-metal server:

   - Resources:
     - CPU:
       - x86_64 architecture;
       - Support for Intel-VT (VMX) or AMD-V (SVM) instructions;
       - At least 4 cores;
     - RAM: At least 8 GB;
     - Disk space:
       - At least 60 GB;
       - High-speed disk (400+ IOPS);
       - Additional disks for software-defined storage;
   - OS [from the list of supported ones](#supported-os-for-platform-nodes);
     - Linux kernel version `5.7` or newer;
   - **Unique hostname** across all servers in the future cluster;
   - Network access:
     - HTTPS access to the container image registry at `registry.deckhouse.io`;
     - Access to the package repositories of the chosen OS;
     - SSH key-based access from the **installation machine** (see p.1);
   - Required software:
     - The `cloud-utils` and `cloud-init` packages must be installed (package names may vary depending on the chosen OS).
   > **Important.** The container runtime will be installed automatically, so do not pre-install any `containerd` or `docker` packages.

1. **Storage hardware**

   Depending on the chosen storage solution, additional resources may be required. For details, refer to the section [Storage Management](/products/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html).

## Supported OS for platform nodes

| Linux distribution          | Supported versions              |
| --------------------------- | ------------------------------- |
| CentOS                      | 7, 8, 9                         |
| Debian                      | 10, 11, 12                      |
| Ubuntu                      | 20.04, 22.04, 24.04      |

## Supported guest operating systems

The virtualization platform supports operating systems running on `x86` and `x86_64` architectures as guest operating systems. For correct operation in paravirtualization mode, `VirtIO` drivers must be installed to ensure efficient interaction between the virtual machine and the hypervisor.

Successful startup of the operating system is determined by the following criteria:

- correct installation and booting of the OS;
- uninterrupted operation of key components such as networking and storage;
- no crashes or errors during operation.

For Linux family operating systems it is recommended to use guest OS images with cloud-init support, which allows initializing virtual machines after their creation.

For Windows operating systems, the platform supports initialization using the built-in sysprep utility.

## Supported virtual machine configurations

Maximum number of cores supported: `254`
Maximum amount of RAM: `1024 GB`

## Supported Storage Systems

Virtual machines use PersistentVolume resources. To manage these resources and allocate disk space within the cluster, one or more supported storage systems must be installed:

| Storage System                              | Disk Location              |
|---------------------------------------------|----------------------------|
| LVM (Logical Volume Manager)                | Local                     |
| DRBD (Distributed Replicated Block Device)  | Replicas on cluster nodes |
| Ceph Cluster                                | External storage          |
| NFS (Network File System)                   | External storage          |
| TATLIN.UNIFIED (Yadro)                      | External storage          |
