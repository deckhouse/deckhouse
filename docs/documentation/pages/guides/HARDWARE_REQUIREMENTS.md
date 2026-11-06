---
title: Picking resources for a bare metal cluster
permalink: en/guides/hardware-requirements.html
description: Hardware requirements for cluster nodes managed by Deckhouse Platform.
lang: en
---

Before deploying a cluster running Deckhouse Platform, you have to plan the configuration of the future cluster and decide on the parameters of its nodes (e.g., RAM, CPU, etc.).

## Installation planning

Before deploying a cluster, you need to plan for the resources that you might need to run the cluster. The following questions will help you plan ahead:

* What is the expected load?
* Does your cluster require a high load mode?
* Does your cluster require a [high availability mode](../glossary.html#ha-mode)?
* Which Deckhouse Platform modules do you intend to use?

The answers to these questions can help you estimate the number of nodes recommended for your cluster deployment. See the [Deployment options](../requirements/#deployment-options) section of the "System requirements" page to learn more.

{% alert level="info" %}
The information below applies to a Deckhouse Platform installation running the [Default module set](../admin/configuration/#module-bundles).
{% endalert %}

## Deployment scenarios

Cluster deployment options ([single-node cluster](../glossary.html#single-node-cluster), cluster with one [master node](../glossary.html#master-node), cluster with three master nodes or cluster with role separation) and node types are described in the [Deployment options](../requirements/#deployment-options) section of the "System requirements" page.

{% alert level="info" %}
Starting with version 1.74, Deckhouse Platform has a module [integrity control](../glossary.html#integrity-control) mechanism that protects modules from being replaced or modified. This mechanism is enabled automatically when the operating system on the nodes where Deckhouse Platform is installed supports the `erofs` kernel module. If this kernel module is not present, Deckhouse Platform will continue to operate without the module integrity control mechanism, but an alert will be displayed indicating that this functionality is not working.
{% endalert %}

## Deciding on the amount of resources needed for nodes

Resource requirements depend on the deployment option (single-node cluster, cluster with one master node, cluster with three master nodes or cluster with role separation) and on the workload type (running containers, virtual machines, or both containers and virtual machines).

The tables with node resources for every deployment option and workload type are given in the [Resource requirements](../requirements/#resource-requirements) section of the "System requirements" page. The same section describes what the values in the tables include and which resources to reserve on nodes in addition.

### Single-node cluster

{% alert level="warning" %}
Such clusters lack fault tolerance. Do not use this kind of clusters in production environments.
{% endalert %}

In some cases, a cluster can consist of a single node that performs all the node roles described above. This configuration may be used, for example, to familiarize yourself with the technology or to run workloads with low resource requirements.

The [Getting Started guide](/products/kubernetes-platform/gs/bm/step5.html) contains instructions for deploying a single-node cluster. Once you un-taint the node, it will run all cluster components included in the selected [module bundle](../glossary.html#module-bundle) ([bundle: Default](/modules/deckhouse/configuration.html#parameters-bundle) by default). The resources for such a node are given in the "Single-node cluster" row of the tables in the [Resource requirements](../requirements/#resource-requirements) section for the required workload type: containers, virtual machines or combined workloads.

Such a configuration would allow some workloads to be run.

In a single-node cluster for running containers with the resources from the table, a load of 2500 RPS on a typical web application (e.g., a static Nginx page) consisting of 30 pods, and incoming traffic of 24 Mbps will result in approximately the following resource consumption figures:

* CPU load will increase to ~60% in total;
* RAM and disk resource consumption figures will remain largely unchanged. In the end, however, it comes down to the number of metrics collected and the nature of the workload being run.

{% alert level="info" %}
Run load testing of the application and adjust the server capacity according to its results.
{% endalert %}

### Configuration examples

The example below is a cluster with role separation without fault tolerance: one node of each type, without dedicated [monitoring nodes](../glossary.html#monitoring-node). The node resources are taken from the [For containers](../requirements/#containers) table for the cluster with role separation.

To deploy Deckhouse Platform for running containers in the [Ultimate/Enterprise Edition](../glossary.html#edition) with the [Default](/modules/deckhouse/configuration.html#parameters-bundle) module set, the following node configuration is required:

* **[master nodes](../glossary.html#master-node)** — 1 node, 6 CPU, 12 GB RAM, 60 GB of disk space;
* **[frontend nodes](../glossary.html#frontend-node)** — 1 node, 2 CPU, 4 GB RAM, 60 GB of disk space;
* **[system nodes](../glossary.html#system-node)** — 1 node, 6 CPU, 12 GB RAM, 300 GB of disk space (the system node hosts the monitoring components and system PVCs);
* **[worker nodes](../glossary.html#worker-node)** — 1 node, 2 CPU, 4 GB RAM, 60 GB of disk space plus the resources for the user workload.

{% alert level="info" %}
If needed, you can also run Deckhouse Platform in the same configuration on a single node. Its resources are given in the "Single-node cluster" row of the same table: 12 CPU, 24 GB RAM and 350 GB of disk space.
{% endalert %}

## Node and network requirements

The requirements for node hardware, the operating system and the network between nodes are given in the [Node requirements](../requirements/#node-requirements) and [Network requirements](../requirements/#network-requirements) sections of the "System requirements" page.

## Community

{% alert level="info" %}
Join our [Telegram channel](https://t.me/deckhouse) to stay up to date.
{% endalert %}

Join the [Deckhouse community](https://deckhouse.io/community/about.html) for updates on important developments and news. There, you will be able to chat with others and learn from their experiences. This way, you can avoid many common mistakes.

Setting up and running a production Kubernetes cluster takes effort. Share your experience with Deckhouse Platform in the community to help others move to Kubernetes.
