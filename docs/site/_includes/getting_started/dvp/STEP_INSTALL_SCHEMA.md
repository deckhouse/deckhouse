This guide walks you through installing Deckhouse Virtualization Platform. It describes a minimal setup with one master node, one worker node, and an external NFS server for virtual machine disks. When you finish, you can explore the platform and deploy a test application.

<div style="text-align: center;">
  <img
    src="/images/virtualization-platform/dvp-architecture-gs.png"
    alt="Deckhouse Virtualization Platform architecture for the getting started guide"
    style="display:block; margin:24px auto; max-width:100%; height:auto;"
  />
</div>

{% alert level="info" %}
This lab configuration is suitable for evaluation only, not for production. Read the [production readiness guide](/products/virtualization-platform/guides/production.html) and [recommended architecture options](/products/virtualization-platform/documentation/about/architecture-options.html) to choose node types and counts for your operational needs.

When you have chosen an architecture, use [Platform installation](/products/virtualization-platform/documentation/admin/install/steps/prepare.html) for detailed production installation steps.
{% endalert %}

## Hardware and software requirements

Installing Deckhouse Virtualization Platform requires the following components to be prepared correctly:

<ol>

  <li><p><strong>Personal computer</strong> — the machine from which you run the installer. It is used only to run the installer and is not part of the cluster.</p>

  {% offtopic title="Requirements..." %}
  - OS: Windows 10+, macOS 10.15+, Linux (Ubuntu 18.04+, Fedora 35+);
  - Docker Engine or Docker Desktop installed ([Ubuntu](https://docs.docker.com/engine/install/ubuntu/), [macOS](https://docs.docker.com/desktop/mac/install/), [Windows](https://docs.docker.com/desktop/windows/install/));
  - HTTPS access to the container image registry `registry.deckhouse.ru`;
  - SSH key-based access to the future cluster **master node**, **worker node**, and **NFS server**.
    {% endofftopic %}
  </li>

  <li><p><strong>Master node</strong> — the cluster control plane node where Deckhouse Virtualization Platform system components run. It manages the cluster, schedules pods, and coordinates all nodes.</p>

  {% offtopic title="Requirements..." %}
  {% alert level="warning" %}
  Later in this guide, `ContainerdV2` is the default container runtime on cluster nodes.
  To use `ContainerdV2`, nodes must meet the following requirements:

  - `CgroupsV2` support;
  - systemd version `244` or newer;
  - `erofs` kernel module support.

  Some distributions (for example, Astra Linux 1.7.4) do not meet these requirements; bring the OS on the nodes into compliance before installing Deckhouse Virtualization Platform. See the [documentation](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri).
  {% endalert %}

  - **CPU**:
    - x86_64 architecture;
    - Intel VT-x (VMX) or AMD-V (SVM) virtualization extensions.
  - **BIOS/UEFI**:
    - Hardware virtualization enabled in firmware settings.
  - **Resources** for a one-master, one-worker cluster:
    - at least 6 vCPU;
    - at least 8 GB RAM;
    - at least 60 GB fast disk space with 400+ IOPS.
  - **Operating system**:
    - a [supported OS](/products/kubernetes-platform/documentation/v1/supported_versions.html);
    - Linux kernel `5.8` or newer.
  - **Software**:
    - `cloud-init` and `cloud-utils` packages installed (names may vary by distribution);
    - no container runtime packages (such as `containerd` or Docker) installed on the node.
  - **Networking**:
    - HTTPS access to `registry.deckhouse.ru` and OS package repositories;
    - SSH access from the personal computer on port `22/TCP` for installation;
    - a **unique hostname** on every cluster node.

  {% alert level="warning" %}
  Stable live migration requires the same Linux kernel version on all cluster nodes.

  Kernel differences can cause incompatible interfaces, syscalls, or resource behavior and break virtual machine migration.
  {% endalert %}
  {% endofftopic %}
  </li>

  <li><p><strong>Worker node</strong> — a worker node for user workloads and virtual machines.</p>

  {% offtopic title="Requirements..." %}
  <p><strong>Worker node</strong> requirements are the same as for the <strong>master node</strong>, and also depend on the workloads you run on the nodes.</p>
  {% endofftopic %}
  </li>

  <li><p><strong>NFS server</strong> — an external Network File System server used for VM disks and cluster component data (metrics, logs, and so on). It provides centralized storage reachable from all cluster nodes.</p>
    {% offtopic title="Requirements..." %}
  - **System requirements**:
    - a [supported OS](/products/kubernetes-platform/documentation/v1/supported_versions.html) with an NFS server package available;
    - sufficient disk space for VM disks.
  - **Access and networking**:
    - NFS access (NFSv4.1 recommended) from master and worker nodes;
    - export the DVP directory with the `no_root_squash` option;
    - SSH key-based access from the **personal computer** (see item 1) for NFS server administration.
  {% endofftopic %}
  </li>

</ol>

## Supported guest operating systems

Deckhouse Virtualization Platform supports guest operating systems on `x86` and `x86-64`. For paravirtualized I/O, install **VirtIO** drivers for efficient communication between the VM and the hypervisor.

A guest OS is considered working when it:

- installs and boots correctly;
- runs core components such as networking and storage reliably;
- does not fail during normal operation.

For Linux guests, use images with **cloud-init** support so VMs can be initialized after creation.

For Windows guests, the platform supports initialization via [unattended setup (autounattend)](https://learn.microsoft.com/en-us/windows-hardware/manufacture/desktop/windows-setup-automation-overview).
