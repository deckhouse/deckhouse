---
title: Picking resources for a bare metal cluster
permalink: en/guides/hardware-requirements.html
description: Hardware requirements for cluster nodes managed by Deckhouse Kubernetes Platform.
lang: en
layout: sidebar-guides
---

Before deploying a cluster running Deckhouse Kubernetes Platform, you have to plan the configuration of the future cluster and decide on the parameters if its nodes (e. g., RAM, CPU, etc.).

## Installation Planning

Before deploying a cluster, you need to plan for the resources that you might need to run the cluster. The following questions will help you plan ahead:

* What is the expected load?
* Does your cluster require a high load mode?
* Does your cluster require a high availability mode?
* Which DKP modules do you intend to use?

The answers to these questions can help you estimate the number of nodes recommended for your cluster deployment. See [Deployment Scenarios](#deployment-scenarios) to learn more.

{% alert level="info" %}
The information below applies to a Deckhouse Kubernetes Platform installation running the [Default module set](/products/kubernetes-platform/documentation/v1/admin/configuration/#module-bundles).
{% endalert %}

## Deployment Scenarios

This section helps you **estimate the resources** required for the cluster based on the expected load.

<table>
  <thead>
    <tr>
      <th>Cluster configuration</th>
      <th style="text-align: center;">Master nodes</th>
      <th style="text-align: center;">Worker nodes</th>
      <th style="text-align: center;">Frontend nodes</th>
      <th style="text-align: center;">System nodes</th>
      <th style="text-align: center;">Monitoring nodes</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td>Minimum</td>
      <td style="text-align: center;">1</td>
      <td style="text-align: center;">at least 1</td>
      <td style="text-align: center;">–</td>
      <td style="text-align: center;">–</td>
      <td style="text-align: center;">–</td>
    </tr>
    <tr>
      <td>Typical</td>
      <td style="text-align: center;">3</td>
      <td style="text-align: center;">at least 1</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">-</td>
    </tr>
    <tr>
      <td>Increased load</td>
      <td style="text-align: center;">3</td>
      <td style="text-align: center;">at least 1</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">2</td>
    </tr>
  </tbody>
</table>

Where:

* **master nodes** — nodes that manage the cluster
* **worker nodes** — these nodes are used to run user applications
* **frontend nodes** — nodes that balance incoming traffic; Ingress controllers run on them
* **system nodes** — these nodes are intended to run Deckhouse modules
* **monitoring nodes** — these nodes are used to run user applications

See [Configuration Features](https://deckhouse.io/products/kubernetes-platform/guides/production.html#things-to-consider-when-configuring) of the "Going to Production" section for details on these node types.

Features of the configurations listed in the table above:

* **Minimum** — Minimum cluster configuration is suitable for small, light-load projects with low reliability requirements. It is up to you to define the characteristics of the worker node based on the expected user load. Note that in this configuration, some of the DKP components will also run on the worker node.
  > Such a cluster configuration is risky because if a single master node fails, the entire cluster will be affected.
* **Typical** — This is the recommended configuration that can tolerate the failure of two master nodes. It greatly improves service availability.
* **Increased load** — Unlike the typical configuration, this configuration includes dedicated monitoring nodes, enabling a high level of observability in the cluster even under high loads.

## Deciding on the amount of resources needed for nodes

<table>
  <thead>
    <tr>
      <th>Requirement level</th>
      <th>Node type</th>
      <th style="text-align: center;">CPU (pcs)</th>
      <th style="text-align: center;">RAM (GB)</th>
      <th style="text-align: center;">Disk space (GB)</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td rowspan="6" style="width: 45%;">
        <b>Minimum</b><br><br>
        <i>The way the cluster will run on minimum requirement nodes largely depends on which DKP modules are enabled.<br>
        We recommend increasing node resources if the number of enabled modules is large.<br><br>
        </i>
      </td>
      <td>Master node</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Worker node</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Frontend node</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">50</td>
    </tr>
    <tr>
      <td>Monitoring node</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>System node</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>System node <i>(if no dedicated monitoring nodes are running in the cluster</i>)</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;"><a href="#storage">60 / 160*</a></td>
    </tr>
    <tr>
      <td rowspan="6" style="width: 45%;">
        <b>Production</b><br><br>
      </td>
      <td>Master node</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">16</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Worker node</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">12</td>
      <td style="text-align: center;">60</td>
    </tr>
    <tr>
      <td>Frontend node</td>
      <td style="text-align: center;">2</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">50</td>
    </tr>
    <tr>
      <td>Monitoring node</td>
      <td style="text-align: center;">6</td>
      <td style="text-align: center;">12</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>System node</td>
      <td style="text-align: center;">4</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;"><a href="#storage">50 / 150*</a></td>
    </tr>
    <tr>
      <td>System node <i>(if no dedicated monitoring nodes are running in the cluster</i>)</td>
      <td style="text-align: center;">8</td>
      <td style="text-align: center;">16</td>
      <td style="text-align: center;"><a href="#storage">60 / 160*</a></td>
    </tr>
    <tr>
      <td style="width: 45%;">
        <b>Single master node cluster</b>
      </td>
      <td>Master node</td>
      <td style="text-align: center;">6</td>
      <td style="text-align: center;">12</td>
      <td style="text-align: center;">160</td>
    </tr>
  </tbody>
</table>

{% alert %}
* <span id="storage"></span>PVC disk space for system components: If the local disk space of the node will be used to store system PVCs (prometheus, upmeter modules, etc.), then it is necessary to additionally allocate >= 100 GB.
* The parameters of worker nodes are largely dictated by the nature of the workload running on the node(s), the table lists the minimum requirements. For system services (kubelet) and system pods on worker nodes, you need to allocate at least 1 CPU and 2 GB of memory.
* Note that all nodes require high performance disks (400+ IOPS).
{% endalert %}

### Single master node cluster

{% alert level="warning" %}
Such clusters lack fault tolerance. We highly advise you against using this kind of clusters in production environments.
{% endalert %}

In some cases, a single-node cluster is enough. In this case, the node will take care of all the node roles described above. For example, this may be useful if you just want to familiarize yourself with the technology or run some fairly lightweight workloads.

The [Getting Started guide](/products/kubernetes-platform/gs/bm/step5.html) contains instructions for deploying a single master node cluster. Once you un-taint the node, it will run all cluster components included in the selected module bundle ([bundle: Default](/modules/deckhouse/configuration.html#parameters-bundle) by default). To successfully run a cluster in this mode, you will need at least 16 CPUs, 32 GB of RAM, and 60 GB of disk space on a performance disk (400+ IOPS). Such a configuration would allow some workloads to be run.

With this configuration, a load of 2500 RPS on a typical web application (e.g., a static Nginx page) consisting of 30 pods, and incoming traffic of 24 Mbps will result in approximately the following resource consumption figures:

- CPU load will increase to ~60% in total
- RAM and disk resource consumption figures will remain largely unchanged. In the end, however, it comes down to the number of metrics collected and the nature of the workload being run

{% alert level="info" %}
We recommend load testing your application and adjusting the server capacity accordingly.
{% endalert %}

## Node Hardware Requirements

The machines you intend to turn into nodes of your future cluster must meet the following requirements:

* **CPU architecture** — all nodes must be of the `x86_64` CPU architecture
* **Identical nodes** — all nodes of the same type must have the same hardware configuration. Nodes must be of the same make and model with the same CPU, memory, and storage
* **Network interfaces** — each node must have at least one network interface for the routed network

## Network Requirements

* Nodes must be able to access each other over the network. The [network policies](../documentation/v1/network_security_setup.html) must be met.
* There are no MTU requirements.
* Each node must have a permanent IP address. If you use a DHCP server to assign IP addresses to nodes, you must configure the DHCP server to explicitly assign addresses to each node. Changing the IP addresses of the nodes is undesirable.
* Master nodes must be able to access time servers external to the cluster via NTP. Cluster nodes use master nodes to synchronize time, but can also synchronize with other time servers (see the [ntpServers](../documentation/v1/modules/chrony/configuration.html#parameters-ntpservers) parameter).

## Community

{% alert %}
Join our [Telegram channel](https://t.me/deckhouse) to stay up to date.
{% endalert %}

Join the [Deckhouse community](https://deckhouse.io/community/about.html) for updates on important developments and news. There, you will be able to chat with others and learn from their experiences. This way, you can avoid many common mistakes.

The Deckhouse team knows firsthand the dedication it takes to set up and orchestrate a production Kubernetes cluster. We're thrilled if Deckhouse empowers you to bring your vision to life. Share your journey and ignite others to embark on their own Kubernetes endeavors!
