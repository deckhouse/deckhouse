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

Deckhouse Virtualization Platform has no additional restrictions and is compatible with any hardware supported by the operating systems on which it can be installed.

## Hardware and software requirements

Hardware requirements for the Deckhouse Virtualization Platform match the requirements for the [Deckhouse Kubernetes Platform](/products/kubernetes-platform/guides/production.html#resource-requirements), with an additional requirement: CPU virtualization support on the hosts where virtual machines will be launched.

### Additional requirements for virtualization support

On all cluster nodes where virtual machines are planned to be launched, hardware virtualization support must be provided:

- CPU: Support for Intel-VT (VMX) or AMD-V (SVM) instructions.
- BIOS/UEFI: Hardware virtualization support enabled in the BIOS/UEFI settings.

{% alert level="warning" %}
Ensuring the stable operation of live migration mechanisms requires using the same Linux kernel version on all cluster nodes.

Differences between kernel versions can lead to incompatible interfaces, system calls, and resource handling, which can disrupt the virtual machine migration process.
{% endalert %}

## Supported guest operating systems

Deckhouse Virtualization Platform supports operating systems running on `x86` and `x86_64` architectures as guest operating systems. For correct operation in paravirtualization mode, `VirtIO` drivers must be installed to ensure efficient interaction between the virtual machine and the hypervisor.

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

Virtual machine disks are created using PersistentVolume resources. To manage these resources and allocate disk space in the cluster, one or more supported storage systems must be deployed:

| Storage System            | Disk Location             |
|---------------------------|---------------------------|
| sds-local-volume          | Local                     |
| sds-replicated-volume     | Replicas on cluster nodes |
| Ceph Cluster              | External storage          |
| NFS (Network File System) | External storage          |
| TATLIN.UNIFIED (Yadro)    | External storage          |
| Huawei Dorado             | External storage          |
| HPE 3par                  | External storage          |
| NetApp                    | External storage          |

## Distribution of components across cluster nodes

The distribution of components across cluster nodes depends on the cluster's configuration. For example, a cluster may consist of:

- Only master nodes, for running the control plane and workload components.
- Only master nodes and worker nodes.
- Master nodes, system nodes, and worker nodes.
- Other combinations (depending on the architecture).

{% alert level="warning" %}
In this context, worker nodes are nodes that do not have taints preventing regular workloads (pods, virtual machines) from running.
{% endalert %}

The table lists the main components of the `virtualization` module control plane and the nodes where they can be placed. Components are scheduled by priority: if a suitable node type is available in the cluster, the component will be placed on it.

| Component name                | Node group        | Comment                               |
|-------------------------------|-------------------|---------------------------------------|
| `cdi-operator-*`              | system/worker     |                                       |
| `cdi-apiserver-*`             | master            |                                       |
| `cdi-deployment-*`            | system/worker     |                                       |
| `virt-api-*`                  | master            |                                       |
| `virt-controller-*`           | system/worker     |                                       |
| `virt-operator-*`             | system/worker     |                                       |
| `virtualization-api-*`        | master            |                                       |
| `virtualization-controller-*` | master            |                                       |
| `virtualization-audit-*`      | system/worker     |                                       |
| `dvcr-*`                      | system/worker     | Storage must be available on the node |
| `virt-handler-*`              | All cluster nodes |                                       |
| `vm-route-forge-*`            | All cluster nodes |                                       |

Components used to create and import virtual machine images or disks (they run only for the duration of the creation or import operation):

| Component name | Node group    | Comment |
|----------------|---------------|---------|
| `importer-*`   | system/worker |         |
| `uploader-*`   | system/worker |         |
