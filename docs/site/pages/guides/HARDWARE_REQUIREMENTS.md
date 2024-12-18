---
title: Hardware requirements for bare metal cluster nodes
permalink: en/guides/hardware-requirements.html
description: Hardware requirements for cluster nodes managed by Deckhouse Kubernetes Platform.
lang: en
---

Before deploying a cluster running Deckhouse Kubernetes Platform, you have to decide on the configuration of the future cluster and select parameters for its nodes (e. g., RAM, CPU, etc.).

## Installation Planning

Before deploying a cluster, you need to plan for the resources that you might need to run the cluster. The following questions will help you plan ahead:

* What is the expected load?
* Does your cluster require a high load mode?
* Does your cluster require a high availability mode?
* Which DKP modules do you intend to use?

{% alert level="info" %}
The information below applies to a Deckhouse Kubernetes Platform installation running the [Default module set](/products/kubernetes-platform/documentation/v1/#module-sets).
{% endalert %}

The answers to these questions can help you estimate the number of nodes recommended for your cluster deployment. See [Deployment Scenarios](#deployment-scenarios) to learn more.

## Deployment Scenarios

{% alert level="warning" %}
This section helps you estimate the resources required for the cluster based on the expected load.
{% endalert %}

In general, your cluster might include the following node types:

* **master nodes** — cluster management nodes
* **frontend nodes** — nodes that balance incoming traffic; Ingress controllers run on them
* **monitoring nodes** — these nodes are used to run Grafana, Prometheus and other monitoring tools
* **system nodes** — these nodes are intended to run Deckhouse modules
* **worker nodes** — these nodes are used to run user applications

See [Configuration Features](https://deckhouse.io/products/kubernetes-platform/guides/production.html#things-to-consider-when-configuring) of the "Going to Production" section for details on these node types.

### Minimum Cluster Configuration

Minimum cluster configuration is suitable for small projects with low load and low reliability requirements. Such a cluster includes:

* **master node** — 1 pc
* **worker nodes** — 1 pc or more

Such a configuration requires **8+ CPUs and 16+GB RAM** as well as speedy **400+ IOPS disks** and a minimum of 50GB for the master nodes.

It is up to you to define the characteristics of the worker node based on the expected user load. Note that in this configuration, some of the DKP components will also run on the worker node.

{% alert level="warning" %}
Such a cluster configuration is risky because if a single master node fails, the entire cluster will be affected.
{% endalert %}

### Typical Cluster Configuration

This is the recommended configuration that can tolerate the failure of two master nodes. It greatly improves service availability.
A typical cluster includes:

* **master nodes** — 3 pcs
* **frontend nodes** — 2 pcs
* **system nodes** — 2 pcs
* **worker nodes** — 1+ pcs depending on the expected load

Such a configuration requires **24+ CPUs and 48+GB RAM** as well as speedy **400+ IOPS disks** and a minimum of 50GB for the master nodes.

{% alert level="info" %}
It is recommended to allocate a dedicated [StorageClass](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) on fast disks for Deckhouse components.
{% endalert %}

### High-load Cluster Configuration

Unlike the typical configuration, this configuration includes dedicated monitoring nodes, enabling a high level of observability in the cluster even under high loads.
The cluster with high load includes:

* **master nodes** — 3 pcs
* **frontend nodes** — 2 pcs
* **system nodes** — 2 pcs
* **monitoring nodes** — 2 pcs
* **worker nodes** — 1+ pcs depending on the expected load

Such a configuration requires **28+ CPUs and 64+GB RAM** as well as speedy **400+ IOPS disks** and a minimum of 50GB for the master and monitoring nodes.

It is recommended to allocate a dedicated [StorageClass](https://deckhouse.io/products/kubernetes-platform/documentation/v1/deckhouse-configure-global.html#parameters-storageclass) on fast disks for Deckhouse components.

## Cluster Node Requirements

Each cluster node must comply with the following requirements:

* [supported OS version](https://deckhouse.io/products/kubernetes-platform/documentation/v1/supported_versions.html) (Limitations apply for some operating systems, see the "Notes" section in the supported versions table.)
* Linux kernel version 5.7 or later
* **unique hostname** within the cluster servers (virtual machines)
* access over HTTP(S) to the `registry.deckhouse.io` container image repository or the corporate registry if installed in a private environment
* access to the default package repositories for the operating system in use
* no container runtime packages (e.g., containerd or Docker) must be present on the node
* the `cloud-utils` and `cloud-init` packages must be installed on the node **and their corresponding services must be running**
* all nodes must have access to time servers (NTP) so that the [chrony](../documentation/v1/modules/chrony/) module can synchronize time

### Minimum Node Resource Requirements

Regardless of the way the cluster will be operated and the workloads expected to be run in it, the following **minimum resources** are recommended for infrastructure nodes depending on their role in the cluster:

* **Master node** — 4 CPUs, 8GB of RAM, 60GB of disk space on a performant disk (400+ IOPS)
* **Frontend node** — 2 CPUs, 4GB of RAM, 50GB of disk space
* **Monitoring node** (for high-load clusters) — 4 CPUs, 8GB of RAM, 50GB of disk space on a performant disk (400+ IOPS)
* **System node**:
  * 2 CPUs, 4GB of RAM, 50GB of disk space — if there are dedicated monitoring nodes in the cluster
  * 4 CPUs, 8GB of RAM, 60GB of disk space on a performant disk (400+ IOPS) — if no dedicated monitoring nodes are running in the cluster
* **Worker node** — the requirements are similar to those for the master node, but largely depend on the nature of the workload running on the node(s).

{% alert level="warning" %}
The way the cluster will run on minimum requirement nodes largely depends on which DKP modules are enabled.

We recommend to increase the node resources if the number of enabled modules is large.
{% endalert %}

### Resource Requirements for Nodes Running Production Workloads

We recommend the following resourses for nodes to ensure that the cluster running production workloads won't run out of resources, and that all modules in any configuration will run smoothly:

* **Master node** — 8 CPUs, 16GB of RAM, 60GB of disk space on a performant disk (400+ IOPS)
* **Frontend node** — 2 CPUs, 4GB of RAM, 50GB of disk space
* **Monitoring node** (for high-load clusters) — 4 CPUs, 8GB of RAM, 50GB of disk space on a performant disk (400+ IOPS)
* **System node**:
  * 6 CPUs, 12GB of RAM, 50GB of disk space — if there are dedicated monitoring nodes in the cluster
  * 8 CPUs, 16GB of RAM, 60GB of disk space on a performant disk (400+ IOPS) — if no dedicated monitoring nodes are running in the cluster
* **Worker node** — 4 CPUs, 12GB of RAM, 60GB of disk space on a performant disk (400+ IOPS)

### Clusters With a Single Master Node

{% alert level="warning" %}
These clusters lack fault tolerance. We highly advise you against using this kind of clusters in production environments.
{% endalert %}

In some cases, a single-node cluster is enough. In this case, the node will take care of all the node roles described above. For example, this may be useful if you just want to familiarize yourself with the technology or run some fairly lightweight tasks.

The [Quick Start Guide](../gs/bm/step5.html) contains instructions for deploying the cluster on a single master node. Once you un-taint the node, it will run all cluster components included in the selected module bundle ([bundle: Default](../documentation/v1/modules/002-deckhouse/configuration.html#parameters-bundle) by default). To successfully run the cluster in this mode, you will need at least 6 CPUs, 12GB of RAM, and 60GB of disk space on a performant disk (400+ IOPS). This configuration will allow some workloads to be run, with Deckhouse consuming roughly ~25% of CPUs, ~9GB of RAM, and ~20GB of disk space.

With this configuration, under a load of 2500 RPS per generic web application (e. g., a static Nginx page) consisting of 30 pods and incoming traffic of 24 Mbps, you can expect the following resource consumption figures:

- CPU load will increase up to ~60% in total
- The RAM and disk numbers mostly stay the same. In the end, however, it all comes down to the amount of metrics collected and the nature of the workload being run

{% alert level="info" %}
We recommend load testing your application and adjusting the server capacity accordingly.
{% endalert %}

## Node Hardware Requirements

The machines that you intend to turn into nodes of your future cluster must meet the following requirements:

* **CPU architecture** — all nodes must be of the `x86_64` CPU architecture
* **Identical nodes** — all nodes of the same type must have the same hardware configuration. Nodes must be of the same make and model with the same CPU, memory, and storage
* **Network interfaces** — each node must have at least one network interface for the routed network

## Network Requirements

Nodes must be able to access each other over the network. The [network policies](../documentation/v1/network_security_setup.html)  must be met.

### Network MTU Requirements

There are no MTU requirements.

### Node IP Address Requirements

Each node must have a permanent IP address.

{% alert level="warning" %}
If you use a DHCP server to assign IP addresses to nodes, you must configure the DHCP server to explicitly assign addresses to each node. Changing the IP addresses of the nodes is undesirable.
{% endalert %}

## Community

{% alert %}
Join the project's [Telegram channel](https://t.me/deckhouse) to stay up to date.
{% endalert %}

Join the [Deckhouse community](https://deckhouse.io/community/about.html) for updates on important developments and news. There, you will be able to chat with others and learn from their experiences. This will help you avoid many of the typical problems.

The Deckhouse team is well aware of the effort required to set up a production cluster in Kubernetes. We would be glad if Deckhouse enables you to fulfill your goals and dreams. Share your experiences and inspire others to migrate to Kubernetes.
