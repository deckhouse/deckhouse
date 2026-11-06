---
title: Cni-flannel module
permalink: en/architecture/network/cni-flannel.html
search: cni-flannel, flannel, vxlan, host-gw
description: Architecture of the cni-flannel module in Deckhouse Platform.
---

The [`cni-flannel`](/modules/cni-flannel/) module provides a network between nodes in a Deckhouse Platform cluster. It is based on the [Flannel](https://github.com/flannel-io/flannel) project and implements the [Container Network Interface (CNI)](https://github.com/containernetworking/CNI).

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/cni-flannel/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`cni-flannel`](/modules/cni-flannel/) module and its interactions with other components of platform are shown in the following diagram:

![Cni-flannel module architecture](../../images/architecture/network/c4-l2-cni-flannel.svg)

## Module components

The module consists of the following components:

1. **Flannel** (DaemonSet): Runs on every cluster node and implements Flannel's Layer 3 overlay network between nodes.

   Depending on the [`podNetworkMode`](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) parameter, it uses the host-gw or VXLAN backend.

   The component takes the pod subnet from the `spec.podCIDR` field of the Node resource and writes the resulting subnet parameters (network, subnet, MTU) to the `/run/flannel/subnet.env` file on the node. The Flannel CNI plugin uses this file on every pod creation or removal.

   It consists of the following containers:

   * **cni-migration-init-checker**: Init container waiting for the migration process from another CNI plugin to complete, if such a migration has been started.
   * **iptables-wrapper-init**: Init container that prepares executables required for working with iptables.
   * **handle-vxlan-offload**: Optional init container that corrects UDP segmentation settings on the node's network interface. It is created by the [Deckhouse controller](../../glossary.html#deckhouse-controller) if the [`podNetworkMode`](/modules/cni-flannel/configuration.html#parameters-podnetworkmode) parameter is set to `VXLAN`.
   * **kube-flannel**: Main container.

1. **Flannel (CNI plugin)**: An executable file run by the containerd component according to the [CNI specification](https://www.cni.dev/docs/spec/#cni-operations), for example ADD when creating a pod's network namespace and DEL when removing it.

   On startup, the component reads the subnet parameters from the `/run/flannel/subnet.env` file and delegates creation of the pod's network interface to the `bridge` CNI plugin, which in turn requests an IP address from the `host-local` IP Address Management ([IPAM](../../glossary.html#sdn-ipam)) plugin. Both `bridge` and `host-local` are standard [Open Source CNI plugins](https://github.com/containernetworking/plugins), installed on every cluster node independently of this module, as part of kubelet installation.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Watches [CNIMigration](../../reference/api/cr.html#cnimigration) and [CNINodeMigration](../../reference/api/cr.html#cninodemigration) custom resources, and Pod and Node resources.
   * Updates the status of the Node resource.

1. **Bridge (CNI plugin)**: Creates the pod's network interface.

The following external components interact with the module:

1. **Containerd**: Runs the flannel executable file with certain commands according to the CNI specification, for example, ADD when creating a pod's network namespace and DEL when removing it.
