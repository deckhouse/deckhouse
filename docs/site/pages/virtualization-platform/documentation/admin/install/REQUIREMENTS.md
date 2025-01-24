---
title: "Requirements"
permalink: en/virtualization-platform/documentation/admin/install/requirements.html
---

{% alert level="warning" %}
Platform components must be installed on physical servers (baremetal servers). For demonstration purposes, installation on virtual machines is allowed, but in this case it is necessary that the nested virtualization function is activated on virtual machines. It is important to emphasize that no technical support is provided for this scenario, which involves the use of virtual machines.
{% endalert %}

## Hardware Requirements

1. A dedicated **machine for installation**.

   This machine will run the Deckhouse installer. For example, it can be an administrator's laptop or any other computer that is not intended to be added to the cluster. Requirements for this machine:

   - OS: Windows 10+, macOS 10.15+, Linux (Ubuntu 18.04+, Fedora 35+);
   - Installed Docker Engine or Docker Desktop (instructions for [Ubuntu](https://docs.docker.com/engine/install/ubuntu/), [macOS](https://docs.docker.com/desktop/mac/install/), [Windows](https://docs.docker.com/desktop/windows/install/));
   - HTTPS access to the container image registry at `registry.deckhouse.io`;
   - SSH key-based access to the node that will serve as the **master node** of the future cluster;
   - SSH key-based access to the node that will serve as the **worker node** of the future cluster (if the cluster will consist of more than one master node).

1. **Server for the master node**

   There can be multiple servers running the cluster's control plane components, for example, 3. Initially, only one server is required for the installation, and additional servers can be added later via the node management mechanisms.

   Requirements for a physical bare-metal server:

   - Resources:
     - CPU:
       - x86_64 architecture;
       - Support for Intel-VT (vmx) or AMD-V (svm) instructions;
       - At least 4 cores.
     - RAM: At least 8 GB.
     - Disk space:
       - At least 60 GB;
       - High-speed disk (400+ IOPS).
   - OS [from the list of supported ones](#supported-os):
     - Linux kernel version `5.7` or newer.
   - **Unique hostname** across all servers in the future cluster;
   - Network access:
     - HTTPS access to the container image registry at `registry.deckhouse.ru`;
     - Access to the package repositories of the chosen OS;
     - SSH key-based access from the **installation machine** (see p.1);
     - Network access from the **installation machine** (see p.1) on port `22322/TCP`.
   - Required software:
     - The `cloud-utils` and `cloud-init` packages must be installed.
   > **Warning.** The container runtime will be installed automatically, so the `containerd` and/or `docker` packages must not be pre-installed.

1. **Servers for worker nodes**

   These are nodes where virtual machines will be run, so the servers must have enough resources to handle the planned number of virtual machines. Additional disks may be required if deploying a software-defined storage solution.

   Requirements for a physical bare-metal server:

   - Resources:
     - CPU:
       - x86_64 architecture;
       - Support for Intel-VT (vmx) or AMD-V (svm) instructions;
       - At least 4 cores;
     - RAM: At least 8 GB;
     - Disk space:
       - At least 60 GB;
       - High-speed disk (400+ IOPS);
       - Additional disks for software-defined storage;
   - OS [from the list of supported ones](#supported-os);
     - Linux kernel version `5.7` or newer;
   - **Unique hostname** across all servers in the future cluster;
   - Network access:
     - HTTPS access to the container image registry at `registry.deckhouse.ru`;
     - Access to the package repositories of the chosen OS;
     - SSH key-based access from the **installation machine** (see p.1);
   - Required software:
     - The `cloud-utils` and `cloud-init` packages must be installed (package names may vary depending on the chosen OS).
   > **Important.** The container runtime will be installed automatically, so the `containerd` and/or `docker` packages must not be installed.

1. **Storage hardware**

   Depending on the chosen storage solution, additional resources may be required. For details, refer to the section [Storage Management](/products/virtualization-platform/documentation/admin/platform-management/storage/sds/lvm-local.html).

## Supported OS for platform nodes

| Linux distribution          | Supported versions              |
| --------------------------- | ------------------------------- |
| CentOS                      | 7, 8, 9                         |
| Debian                      | 10, 11, 12                      |
| Ubuntu                      | 20.04, 22.04, 24.04      |

## Supported guest operating systems

The virtualization platform supports operating systems running on x86 and x86_64 architectures as guest operating systems. In order for paravirtualization mode to function successfully, the appropriate virtio drivers must be installed to ensure efficient communication between the virtual machine and the hypervisor.

A successful startup of the operating system is determined by the correct installation and booting of the system, the smooth functioning of all major components such as networking and storage, and the absence of failures or errors.

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
