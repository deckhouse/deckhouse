---
title: Sdn module
permalink: en/architecture/network/sdn.html
search: sdn, software-defined networking, cni
description: Architecture of the sdn module in Deckhouse Platform.
---

The [`sdn`](/modules/sdn/) module provides software-defined networking (SDN) functions in Deckhouse Platform (DP):

- Configuring network interfaces on nodes.
- Additional networks for pods and virtual machines.
- IPAM for additional networks.
- Underlay networks for passing through hardware network devices.
- System networks for node-level service traffic.

The module works with the following custom resources of the `network.deckhouse.io` API group:

- [ClusterIPAddress](/modules/sdn/cr.html#clusteripaddress): Represents a request for an IPv4 address at the cluster scope, for a network interface connected to a ClusterNetwork or SystemNetwork.
- [ClusterIPAddressPool](/modules/sdn/cr.html#clusteripaddresspool): Defines a cluster-scoped pool of IPv4 addresses used for IPAM in cluster networks.
- [ClusterNetwork](/modules/sdn/cr.html#clusternetwork): Defines a publicly available network at the cluster scope.
- [IPAddress](/modules/sdn/cr.html#ipaddress): Represents a request for an IPv4 address for an additional network interface of a pod connected to a project network.
- [IPAddressLease](/modules/sdn/cr.html#ipaddresslease): A reservation record for an allocated IP address that prevents it from being assigned again.
- [IPAddressPool](/modules/sdn/cr.html#ipaddresspool): Defines a pool of IPv4 addresses within a user namespace, used for IPAM in a project network.
- [Network](/modules/sdn/cr.html#network): A namespaced resource that creates a dedicated project network based on a network class provided by the administrator in a NetworkClass resource.
- [NetworkClass](/modules/sdn/cr.html#networkclass): Defines a range of VLAN tags and node network interfaces that users can base their own Network resources on.
- [NodeNetworkInterface](/modules/sdn/cr.html#nodenetworkinterface): Describes a network interface on a node, used both to discover existing interfaces and to create new ones (VLAN, Bond).
- [SystemNetwork](/modules/sdn/cr.html#systemnetwork): Creates an additional service network on cluster nodes on top of an existing underlay network for node-level service traffic.
- [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork): Provides direct pass-through of physical and virtual node network interfaces (PF/VF) to pods via Dynamic Resource Allocation (DRA), in Shared or Dedicated mode.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/sdn/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`sdn`](/modules/sdn/) module and its interactions with other components of DP are shown in the following diagram:

![Sdn module architecture](../../images/architecture/network/c4-l2-sdn.svg)

## Module components

The module consists of the following components:

1. **Controller** (Deployment): the controller manages software-defined networks in DP, performing the following actions:

   * Manages custom resources of the `network.deckhouse.io` API group.
   * Serves the admission webhooks that validate and mutate them via the [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.
   * Mutates Pod resources in namespaces labeled `direct-nic-access.network.deckhouse.io/enabled` to inject a ResourceClaim for hardware network device pass-through.

   It consists of a single **controller** container.

1. **Agent** (DaemonSet): a component that runs on every cluster node and performs the following operations:

   * Configures node network interfaces (port bonding, bridging, VLAN interface configuration).
   * Brings up additional, underlay, and system networks.
   * Implements a CNI server that the d8-sdn component connects to over a Unix socket.
   * Implements a Dynamic Resource Allocation (DRA) driver for the `network.deckhouse.io` API group, registering itself as a DRA plugin with kubelet in editions where the UnderlayNetwork resource is available, for passing physical and virtual network devices through to a pod.

   The agent translates cluster node and pod configurations into low-level network interface and eBPF program settings that actually process traffic.

   It consists of the following containers:

   - **install-cni-plugin**: Init container that copies the `d8-sdn` CNI plugin executable to the `/opt/cni/bin` directory on the node.
   - **agent**: Main container.

   {% alert level="warning" %}
   The agent component has privileged access to the operating system of each node. On Linux, this requires the following capabilities:

   - NET_ADMIN
   - SYS_ADMIN
   - BPF

   The container also runs as the root user (`runAsUser: 0`), uses `hostNetwork: true`, and the `Unconfined` AppArmor profile. This is required to configure node network interfaces, network namespaces, and to load BPF programs.
   {% endalert %}

1. **D8-sdn**: A CNI plugin executable run by the containerd component according to the [CNI specification](https://www.cni.dev/docs/spec/#cni-operations), for example, ADD when creating a pod's network namespace and DEL when removing it. To configure a pod's network interfaces, d8-sdn sends a request to the agent's API over a Unix socket and triggers the low-level network configuration for the pod.

1. **Scheduler** (Deployment): a kube-scheduler extender that suggests nodes for pod placement according to the pods' network interface settings.

1. **Cni-plugin-cleaner** (DaemonSet): A Helm pre-delete hook. It runs on every node when the module is removed and deletes the CNI plugin files (the executable in `/opt/cni/bin` and the configuration in `/etc/cni/net.d`) installed by the agent component. It does not take part in the module's normal operation.

1. **Cni-plugin-cleaner-waiter** (Job): A Helm pre-delete hook. It waits for the `cni-plugin-cleaner` DaemonSet to finish on every cluster node before Helm is allowed to complete removing the module release.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Manages custom resources of the `network.deckhouse.io` API group.
   * Manages Secret, Pod, Node, Namespace, DeviceClass, ResourceSlice, ResourceClaim, and ResourceClaimTemplate resources.

1. **Kubelet**: Registers itself as a DRA plugin for resources of the `network.deckhouse.io` API group.

The following external components interact with the module:

1. **Kube-apiserver**:

   * Calls validation for custom resources of the `network.deckhouse.io` API group.
   * Calls mutation for NodeNetworkInterface and Pod resources.

1. **Kube-scheduler**: Sends requests to the `sdn-scheduler-extender` webhook for scheduling pods that use software-defined networks.

1. **Containerd**: Runs the d8-sdn executable file with certain commands according to the CNI specification, for example, ADD when creating a pod's network namespace and DEL when removing it.

1. **Kubelet**: Calls the `PrepareResourceClaims` and `UnprepareResourceClaims` RPC-methods to configure a pod's DRA resources.
