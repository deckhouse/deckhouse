---
title: Hybrid Clusters
permalink: en/admin/integrations/hybrid/overview.html
lang: en
search: hybrid, hybrid cluster, hybrid integration
description: General principles of hybrid integration in Deckhouse Kubernetes Platform.
---

A hybrid cluster is a static Deckhouse Kubernetes Platform (DKP) cluster extended with nodes placed in the infrastructure of a supported provider, such as Yandex Cloud, VMware Cloud Director or VMware vSphere.

In this scenario, the control plane and the initial cluster nodes remain part of the static DKP cluster, while additional nodes are added through the corresponding provider module. These nodes can be created automatically through the provider API or connected manually if the virtual machines were created in advance.

This approach allows you to expand an existing static cluster without creating a separate Kubernetes cluster: increase compute capacity, place some workloads in another infrastructure, or gradually migrate services there. Applications still use a single Kubernetes control plane: the same API, shared resources, and unified mechanisms for scheduling, monitoring, updates, and operations.

Hybrid integration is performed on top of an already deployed static DKP cluster ([`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype)). Then the corresponding cloud provider module is enabled, allowing DKP to obtain information about the external infrastructure and create or connect additional nodes.

Depending on the provider, nodes can be added automatically through the provider API or connected manually by running a bootstrap script on pre-created virtual machines.

The general principles of hybrid integration are described on this page. The procedure for preparing the infrastructure and adding nodes depends on the provider being used and is described in separate guides:

- [Yandex Cloud](./yandex-hybrid.html);
- [VMware Cloud Director](./vcd-hybrid.html);
- [VMware vSphere](./vsphere-hybrid.html).

## NodeGroup types

In a hybrid scenario, DKP uses the following NodeGroup types:

- [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html) — nodes that DKP creates and deletes automatically through the configured cloud provider API.
- [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html) — nodes that are created manually by the user or by external tools in the same cloud infrastructure for which cloud provider integration is configured. CSI runs on such nodes, and the Node object is managed by cloud-controller-manager: it is automatically enriched with zone and region information based on provider data.

## Ways to add nodes

Nodes from the provider infrastructure can be connected in the following ways:

- **Automatic creation of cloud nodes**. DKP creates virtual machines through the provider API. VM parameters are described by the `*InstanceClass` resource, while the required number of nodes and placement zones are defined by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource with the `CloudEphemeral` type.
- **Connecting manually created cloud nodes using a bootstrap script**. A virtual machine is created in advance by the user in the provider infrastructure, after which a DKP bootstrap script is run on it to connect the node to the cluster. This scenario uses a [NodeGroup](/modules/node-manager/cr.html#nodegroup) with the `CloudStatic` type. Such a node is managed by the corresponding provider's cloud-controller-manager.

This section describes the general network requirements for hybrid clusters. Provider-specific requirements, including network parameters, virtual machine templates, credentials, placement parameters, and ways to add nodes, are described in separate guides for [Yandex Cloud](./yandex-hybrid.html), [VCD](./vcd-hybrid.html), and [vSphere](./vsphere-hybrid.html).

## General network requirements

Two-way network connectivity sufficient for DKP and Kubernetes components must be configured between the nodes of the initial static cluster and the connected nodes placed in the provider infrastructure.

Connectivity must provide:

- access from connected nodes to the Kubernetes API of the initial cluster;
- access from connected nodes to DNS servers;
- access from connected nodes to the container image registry and other external services required to download images and packages;
- access from the control plane and system components to the connected nodes for kubelet, networking components, monitoring, and Kubernetes service operations;
- access from DKP components that interact with the provider infrastructure to the corresponding provider API.

The full list of required connections is provided in the [Network interaction](../../../../reference/network_interaction.html) section, and access restriction recommendations are provided in the [Configuring network policies](../../configuration/network/policy/configuration.html) section.

Additionally, it is recommended to check:

- routing between the networks of the initial static cluster and the connected nodes in both directions;
- consistent MTU values across the entire network path, especially when tunnels are used;
- traffic encapsulation parameters when using Cilium, including [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), if traffic filtering is applied between sites.

Specific requirements for networks, subnets, virtual machine templates, credentials, and additional parameters depend on the infrastructure provider being used and are described in the "Prerequisites" section for the corresponding provider.
