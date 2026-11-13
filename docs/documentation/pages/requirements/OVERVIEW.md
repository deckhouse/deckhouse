---
title: "System requirements"
permalink: en/requirements/
description: |
  Deckhouse Platform installation requirements and infrastructure preparation.
lang: en
search: Deckhouse Platform resource requirements, system requirements, installation, platform setup, infrastructure preparation, installer configuration
extractedLinksMax: 0
extractedLinksOnlyMax: 0
relatedLinks:
  - title: "Getting started"
    url: /products/kubernetes-platform/gs/
  - title: "Disk layout and size"
    url: ../guides/fs-requirements.html
  - title: "Going to production"
    url: ../guides/production.html
  - title: "Installing Deckhouse Platform in a private environment"
    url: ../guides/private-environment.html
  - title: "Supported Kubernetes and OS versions"
    url: ../reference/supported_versions.html
  - title: "Network interaction of the platform components"
    url: ../reference/network_interaction.html
---

This page describes the requirements for cluster nodes and the environment that must be met before installing Deckhouse Platform.

Before the installation, check the following:

- The [deployment option](#deployment-options) is chosen: how many nodes the cluster has, how roles are distributed between them, and whether the cluster tolerates a node failure.
- Nodes have the CPU, memory and disks from the [tables](#resource-requirements) for the chosen deployment option and workload type: [running containers](#containers), [running virtualization](#virtual-machines) or [combined workloads](#combined).
- Nodes have fast disks (400+ IOPS), and the disk layout follows the [disk layout and size recommendations](../guides/fs-requirements.html).
- Nodes meet the [node requirements](#node-requirements): CPU architecture, operating system, kernel version and installed software.
- For an installation into an existing Kubernetes cluster, its version is in the [list of supported Kubernetes versions](../reference/supported_versions.html#kubernetes), and the node OS is in the [list of supported OS](../reference/supported_versions.html#linux).
- The network between nodes meets the [network requirements](#network-requirements): nodes have permanent IP addresses, the required ports are open, latency between [master nodes](../glossary.html#master-node) is within limits, and a domain for the platform web interfaces is prepared.
- If the cluster will run virtual machines, nodes and the infrastructure meet the [requirements for running virtual machines](#requirements-for-running-virtual-machines).
- There is [access to the infrastructure](#access-to-the-infrastructure): to the container image registry, to nodes over SSH or to the [cloud provider](../glossary.html#cloud-provider) API, and the [installer](../glossary.html#installer) host is ready to run the installation.

## Deployment options

The [deployment option](../glossary.html#deployment-scenario) defines how many nodes the cluster has, how roles are distributed between them, and whether the cluster tolerates a node failure.

A cluster can have nodes of the following types:

- **[Master nodes](../glossary.html#master-node)** — nodes that run the [control plane](../glossary.html#control-plane) components and manage the cluster.
- **[Worker nodes](../glossary.html#worker-node)** — nodes for the user workload: containerized applications and virtual machines.
- **[System nodes](../glossary.html#system-node)** — nodes for Deckhouse Platform modules.
- **[Frontend nodes](../glossary.html#frontend-node)** — nodes that balance incoming traffic. [Ingress controllers](../glossary.html#ingress-controller) run on them.
- **[Monitoring nodes](../glossary.html#monitoring-node)** — nodes for Prometheus, Grafana and other monitoring components.

For details on configuring nodes of each type, see [Things to consider when configuring](../guides/production.html#things-to-consider-when-configuring) in the "Going to production" guide.

Examples of deployment options:

- **[Single-node cluster](../glossary.html#single-node-cluster)** (Single Node / Edge). One node performs all roles: it runs the control plane, Deckhouse Platform modules and the user workload. This option suits getting acquainted with the platform, test environments and edge scenarios.

  {% alert level="warning" %}
  A single-node cluster is not fault-tolerant. Do not use it in production environments.
  {% endalert %}

- **[Single-master cluster](../glossary.html#single-master)**. One master node manages the cluster, and the user workload runs on one or more worker nodes. Some Deckhouse Platform components also run on worker nodes. This option suits small projects with a low load. The cluster is not fault-tolerant: if the master node fails, the entire cluster is affected.
- **High-availability (HA) cluster with three master nodes**. The control plane is distributed across three master nodes and keeps working if one of them fails: the two remaining nodes keep the [quorum](../glossary.html#quorum). The user workload can run on master nodes or on dedicated worker nodes. This is the recommended option for production environments. To provide high availability with limited resources, you can use an [arbiter node](../admin/configuration/high-reliability-and-availability/enable.html#configuring-ha-mode-with-two-master-nodes-and-an-arbiter-node) instead of one of the master nodes.
- **Cluster with role separation**. The control plane runs on three dedicated master nodes, Deckhouse Platform modules run on system nodes, and Ingress controllers run on frontend nodes. The user workload runs only on worker nodes and is isolated from [system components](../glossary.html#system-component). In loaded clusters with a large volume of metrics, monitoring components are moved to dedicated monitoring nodes; without them, they run on system nodes. This option suits large and loaded clusters.

The node resource requirements for these options are listed in the [Resource requirements](#resource-requirements) section.

## Resource requirements

Resource requirements depend on the [deployment option](#deployment-options) and on the solution you choose for the infrastructure tasks (running containers, virtual machines, or both containers and virtual machines).

{% alert level="info" %}
The requirements below are given per node and cover only the load of system components. Allocate resources for the user workload in addition to them.
{% endalert %}

When choosing resources, consider the following:

- The values are calculated for an installation with the `Default` [module bundle](../admin/configuration/#module-bundles).
- The values in the "Disk, GB" column include 100 GB for system PVCs (of the `prometheus`, `upmeter` modules and others) on the node's local disk. If system PVCs are stored in an [external storage](../glossary.html#external-storage), this space is not needed on the node.
- On every [worker node](../glossary.html#worker-node), reserve at least 1 CPU and 2 GB of memory for system services (`kubelet`) and system pods.
- For all nodes, use fast disks with performance of at least 400 IOPS.
- An [arbiter node](../glossary.html#arbiter-node) in an HA cluster with two master nodes needs at least 2 CPU, 4 GB of memory and 8 GB of disk for etcd. For details, see the [Configuring HA mode with two master nodes and an arbiter node](../admin/configuration/high-reliability-and-availability/enable.html#configuring-ha-mode-with-two-master-nodes-and-an-arbiter-node) section.

For details on resource planning, see the guides:

- [Picking resources for a bare metal cluster](../guides/hardware-requirements.html);
- [Disk layout and size](../guides/fs-requirements.html);
- [Going to production](../guides/production.html).

Select an option depending on the goals of the Deckhouse Platform deployment:

{% tabs requirements %}
{% tab "For containers" %}

{: #containers .anchored}

The table lists the Deckhouse Platform resource requirements for running containers, depending on the deployment option:

{% include admin/requirements-table.liquid data="dp-containers-requirements" %}

{% endtab %}
{% tab "For virtualization" %}

{: #virtual-machines .anchored}

The table lists the Deckhouse Platform resource requirements for running virtual machines, depending on the deployment option:

{% include admin/requirements-table.liquid data="dp-virtualization-requirements" %}

Also consider the [requirements for running virtual machines](#requirements-for-running-virtual-machines).

{% endtab %}
{% tab "For combined workloads" %}

{: #combined .anchored}

The table lists the Deckhouse Platform resource requirements for combined workloads (running containers and virtual machines), depending on the deployment option:

{% include admin/requirements-table.liquid data="dp-combined-requirements" %}

Also consider the [requirements for running virtual machines](#requirements-for-running-virtual-machines).

{% endtab %}
{% endtabs %}

Before the installation, the [installer](../glossary.html#installer) checks that the server (VM) for the first [master node](../glossary.html#master-node) has the minimum resources. These values are lower than those in the tables: they are enough for the installation to start, but not for running the cluster under load.

{% offtopic title="Minimum resources checked by the installer..." %}
The server (VM) for the first master node must have:

- At least 4 CPU cores (at least 2 CPU cores with the `Minimal` [module bundle](../admin/configuration/#module-bundles)).
- At least 8 GB of RAM (at least 4 GB with the `Minimal` [module bundle](../glossary.html#module-bundle)).
- At least 50 GB of disk space with 400+ IOPS performance.

The other installer checks are described in the [Pre-installation checks](../installing/#pre-installation-checks) section.
{% endofftopic %}

## Node requirements

The servers (VMs) for cluster nodes must meet the following requirements.

Hardware:

- **CPU architecture** — `x86_64` on all nodes.
- **Identical nodes** — all nodes of the same type have the same configuration: the same make and model with the same CPU, memory and storage.
- **Network interfaces** — each node has at least one network interface for the routed network.

Operating system and software:

- An OS [from the list of supported ones](../reference/supported_versions.html#linux) or a compatible one is installed.
- Linux kernel version 5.8 or newer.
- One of the package managers is installed: `apt`, `apt-get`, `yum`, or `rpm`. For RED OS, `yum` and `which` are installed (they may be missing by default).
- Access to standard OS package repositories for installing dependencies.
- Python and the `sudo` command are available: the installer needs them.
- The hostname is unique within the cluster and meets the Kubernetes requirements: no more than 63 characters, lowercase letters only, hyphens and periods are allowed but not at the beginning or end of the name.
- The node resolves its own hostname, and `localhost` resolves to `127.0.0.1`.
- The system time is correct.
- The node has no container runtime of its own (for example, containerd or Docker) and no Kubernetes components.
- The local `deckhouse` user and group are either absent or have UID and GID `64535`, and the user has no `sudo` privileges. For details, see the [The local deckhouse system user](../installing/#the-local-deckhouse-system-user) section.
- **When using `ContainerdV2`** as the default container runtime on cluster nodes:
  - Support for `CgroupsV2`.
  - Systemd version `244` or newer.
  - Support for the `erofs` kernel module.
  - The Linux kernel version is not in the 6.12.0–6.12.28 or 6.14.0–6.14.6 ranges (these versions are affected by the CVE-2025-37999 vulnerability in EROFS).

The installer checks these requirements before the installation starts. The full list of checks is given in the [Pre-installation checks](../installing/#pre-installation-checks) section.

If security policies or security scanners apply to the nodes, take into account the [sysctl parameters](../reference/sysctl.html) that the platform sets on nodes and the [directories used by the platform](../reference/used_directories.html).

## Network requirements

The following requirements apply to the network between cluster nodes:

- Nodes can reach each other over the network. There are no MTU requirements.
- Each node has a permanent IP address. If a DHCP server assigns the addresses, configure it to assign a fixed address to each node. Changing the IP address of a node is undesirable.
- Master nodes can access external time sources via NTP. The other nodes synchronize time with master nodes, but can also use other time servers (the [`ntpServers`](/modules/chrony/configuration.html#parameters-ntpservers) parameter).
- If the infrastructure restricts network interaction, the ports from the tables in the [Network interaction of the platform components](../reference/network_interaction.html) section are open between nodes.
- The sum of the round-trip time (RTT) and jitter between the servers for master nodes and [arbiter nodes](../glossary.html#arbiter-node) does not exceed 100 ms, or 50 ms when planning the cluster. For details, see the [Requirements for network latency between master nodes](../reference/network_interaction.html#requirements-for-network-latency-between-master-nodes) section.
- A domain for the platform web interfaces is prepared; it is set in the [DNS name template](../glossary.html#public-domain-template): a wildcard record pointing to the [Ingress controller](../glossary.html#ingress-controller), or an A or CNAME record for every name from the [list of reserved DNS names](../reference/public-domains.html).

## Requirements for running virtual machines

{% alert level="warning" %}
Nodes that run virtual machines have to be physical servers (bare metal). Installation on virtual machines is allowed for demonstration purposes only, requires nested virtualization to be enabled, and isn't covered by technical support.
{% endalert %}

Nodes and infrastructure for running VMs have to meet additional requirements. Take them into account when planning the cluster, because some settings can't be changed after the installation:

- A CPU supporting Intel-VT (VMX) or AMD-V (SVM) instructions, with hardware virtualization enabled in the BIOS/UEFI settings.
- The same Linux kernel version on all cluster nodes.
- Astra Linux nodes require Astra Linux version 1.8.3 or later: earlier versions contain a bug that interferes with virtualization.
- One or several [supported storages](../admin/configuration/storage/) in the cluster. Also set the [default StorageClass](../admin/configuration/storage/supported-storage.html#setting-a-default-storageclass).
- The subnets that VMs will get IP addresses from (set in the [`virtualMachineCIDRs`](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) parameter) must not overlap the pod ([`podSubnetCIDR`](/modules/control-plane-manager/configuration.html#parameters-network-podsubnetcidr)) and service ([`serviceSubnetCIDR`](/modules/control-plane-manager/configuration.html#parameters-network-servicesubnetcidr)) address spaces.

A cluster with virtualization is designed to run up to `1000` nodes and up to `50000` virtual machines.

## Access to the infrastructure

The installation requires the following access:

- Access to the Deckhouse Platform container image registry: the public one (`registry.deckhouse.io` or `registry.deckhouse.ru`) or its mirror.
- For an installation on bare metal (including a hybrid cluster) or in an unsupported cloud — access to nodes over SSH with a key.
- For an installation of a [hybrid cluster](../glossary.html#hybrid-cluster) — network connectivity between the sites according to the [general network requirements for hybrid clusters](../admin/integrations/hybrid/overview.html#general-network-requirements).
- For an installation in a supported cloud — quotas for creating resources and the cloud infrastructure access parameters. They depend on the provider and are described in the [Infrastructure integration](../admin/integrations/integrations-overview.html) section.
- For an installation into an existing Kubernetes cluster — access to the Kubernetes API of the cluster from the installer host with administrator rights (kubeconfig). Installation directly from a [master node](../glossary.html#master-node) is not supported.
- On the host that runs the [installer](../glossary.html#installer) — a container runtime for running the installer container, access to the container image registry, and network access to the master node server on port `22/TCP`.

The requirements for the [bastion host](../glossary.html#bastion-host) and the image registry in an air-gapped environment are listed in the [Installing Deckhouse Platform in an air-gapped environment](../guides/private-environment.html) guide.
