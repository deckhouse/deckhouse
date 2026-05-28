---
title: Hybrid integrations
permalink: en/admin/integrations/hybrid/overview.html
search: hybrid, hybrid cluster, hybrid integration
description: General principles of hybrid integration in Deckhouse Kubernetes Platform.
---

A hybrid cluster is a Deckhouse Kubernetes Platform (DKP) cluster in which the control plane and base worker nodes are placed in your own infrastructure, while additional worker nodes are connected from an external cloud or virtualization environment, for example from Yandex Cloud, VMware Cloud Director, or VMware vSphere.

This approach allows you to extend an existing static cluster without creating a separate Kubernetes cluster: increase compute capacity, place part of the workloads in external infrastructure, or gradually migrate services there. For applications, a single Kubernetes control plane is preserved: a common API, common resources, and unified mechanisms for scheduling, monitoring, updating, and operations.

Hybrid integration is performed on the basis of an already deployed static DKP cluster with the [`clusterType: Static`](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-clustertype) parameter. Then the corresponding cloud provider module is enabled, through which DKP receives information about the external infrastructure and can create or connect additional worker nodes.

Depending on the provider, nodes can be added automatically through the cloud API or by manually connecting pre-created virtual machines: using the DKP bootstrap script or Cluster API Provider Static (CAPS).

The general principles of hybrid integration are described on this page. The procedure for preparing the infrastructure and adding worker nodes depends on the provider being used and is provided in separate guides:

- [Yandex Cloud](./yandex-hybrid.html)
- [VMware Cloud Director](./vcd-hybrid.html)
- [VMware vSphere](./vsphere-hybrid.html)

## Node group types

DKP uses different node group types for hybrid scenarios:

- [`Static`](../../../../architecture/cluster-and-infrastructure/node-management/static-nodes.html): Nodes that are created and maintained by the user; also used in scenarios involving connection through CAPS.
- [`CloudEphemeral`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html): Nodes that DKP creates and deletes automatically through the provider API.
- [`CloudStatic`](../../../../architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html): Nodes that are created by the user in external infrastructure and then connected to the cluster.
- `Hybrid`: The NodeGroup type used in the scenario for connecting manually created cloud nodes in Yandex Cloud.

## Node addition methods

Worker nodes from external infrastructure can be connected in several ways:

- **Automatic node creation**. DKP creates virtual machines through the provider API. VM parameters are described by the `*InstanceClass` resource, while the required number of nodes and placement zones are described by the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource.
- **Connecting manually created nodes through CAPS**. A virtual machine is created by the user in advance, and DKP connects to it over SSH through Cluster API Provider Static and configures the node. This uses the [NodeGroup](/modules/node-manager/cr.html#nodegroup), [SSHCredentials](/modules/node-manager/cr.html#sshcredentials), and [StaticInstance](/modules/node-manager/cr.html#staticinstance) resources.
- **Connecting manually created nodes through a bootstrap script**. A virtual machine is created by the user in advance, after which the DKP bootstrap script is run on it to connect the node to the cluster.

This section describes the general network requirements for hybrid clusters. Provider-specific requirements for networks, virtual machine templates, credentials, placement parameters, and node addition methods are provided in separate guides for [Yandex Cloud](./yandex-hybrid.html), [VCD](./vcd-hybrid.html), and [vSphere](./vsphere-hybrid.html).

## General network requirements

Bidirectional network connectivity sufficient for DKP and Kubernetes components to operate must be configured between the cluster’s static nodes and nodes placed in external infrastructure.

Connected nodes must have access to the Kubernetes API, DNS, and the required external service addresses, including the container registry. In the reverse direction, the control plane and cluster system components must have access to the connected nodes for kubelet, network components, monitoring, and Kubernetes service operations.

DKP components that interact with external infrastructure must have access to the API of the corresponding provider.

The full list of connections is provided in the [Network interaction](../../../../reference/network_interaction.html) section, and access restriction recommendations are provided in the [Network policy configuration](../../configuration/network/policy/configuration.html) section.

It is also recommended to check:

- Routing between the networks of static and connected nodes in both directions
- Availability of the Kubernetes API for connected nodes
- Availability of connected nodes from the control plane and cluster system components
- Availability of DNS servers and allowed external addresses
- Availability of the container registry and other external services required for downloading images and packages
- The same MTU value along the entire network path, especially when using tunnels
- Traffic encapsulation parameters when using Cilium, including [`tunnelMode`](/modules/cni-cilium/configuration.html#parameters-tunnelmode), if traffic filtering is applied between sites

Specific requirements for networks, subnets, virtual machine templates, credentials, and additional parameters depend on the infrastructure provider being used and are provided in the "Prerequisites" section for the corresponding provider.
