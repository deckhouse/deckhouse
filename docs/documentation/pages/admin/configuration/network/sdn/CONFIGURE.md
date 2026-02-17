---
title: "Configuring SDN in a cluster"
permalink: en/admin/configuration/network/sdn/configure.html
description: |
  Preparing the cluster for use with software defined networking.
search: software-defined networks, VLAN interfaces, additional networks, underlay networks
---

To use SDN in a DKP cluster, you need to prepare the infrastructure for enabling the [`sdn`](/modules/sdn/) module, as well as perform some preparatory actions after enabling it.

## Preparing the infrastructure for enabling the `sdn` module

Before using additional software-defined networks (hereinafter referred to as additional networks) in a cluster, preliminary infrastructure preparation is required:

* **For creating additional networks based on tagged VLANs:**
  * Allocate VLAN ID ranges on the data center switches and configure them on the corresponding switch interfaces.
  * Select physical interfaces on the nodes for subsequent configuration of tagged VLAN interfaces. You can reuse interfaces already used by the DKP local network.

* **For creating additional networks based on direct, untagged access to a network interface:**
  * Reserve separate physical interfaces on the nodes and connect them into a single local network at the data center level.

## Enabling the `sdn` module

Enable the `sdn` module according [to the instructions](/modules/sdn/configuration.html).

## Actions after enabling the sdn module

After enabling the module, [NodeNetworkInterface](/modules/sdn/cr.html#nodenetworkinterface) objects will automatically appear in the cluster, reflecting the current state of the nodes.

To check for resources, use the command:

```shell
d8 k get nodenetworkinterface
NAME                            MANAGEDBY   NODE           TYPE     IFNAME           IFINDEX   STATE      AGE
virtlab-ap-0-nic-1c61b4a68c2a   Deckhouse   virtlab-ap-0   NIC      eth1             3         Up         35d
virtlab-ap-0-nic-fc34970f5d1f   Deckhouse   virtlab-ap-0   NIC      eth0             2         Up         35d
virtlab-ap-1-nic-1c61b4a6a0e7   Deckhouse   virtlab-ap-1   NIC      eth1             3         Up         35d
virtlab-ap-1-nic-fc34970f5c8e   Deckhouse   virtlab-ap-1   NIC      eth0             2         Up         35d
virtlab-ap-2-nic-1c61b4a6800c   Deckhouse   virtlab-ap-2   NIC      eth1             3         Up         35d
virtlab-ap-2-nic-fc34970e7ddb   Deckhouse   virtlab-ap-2   NIC      eth0             2         Up         35d
```

{% alert level="info" %}
When discovering node interfaces, the controller affixes the following labels, which are service labels (example):

```yaml
labels:
  network.deckhouse.io/interface-mac-address: fa163eebea7b
  network.deckhouse.io/interface-type: NIC
  network.deckhouse.io/nic-pci-bus-info: 0000-17-00.0
  network.deckhouse.io/nic-pci-type: PF
  network.deckhouse.io/node-name: worker-01
annotations:
  network.deckhouse.io/heritage: NetworkController
```

{% endalert %}

In this example, each cluster node has two network interfaces: eth0 (DKP local network) and eth1 (dedicated interface for additional networks).

### Marking interfaces for organizing additional software-defined networks

To enable the configuration of [additional software-defined networks](#configuring-and-connecting-additional-virtual-networks-for-use-in-application-pods), label the dedicated interfaces that are planned to be used for network creation (in the example above, eth1) with an appropriate label:

```shell
d8 k label nodenetworkinterface virtlab-ap-0-nic-1c61b4a68c2a nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-1-nic-1c61b4a6a0e7 nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-2-nic-1c61b4a6800c nic-group=extra
```

### Combining multiple physical interfaces into a channel aggregation interface (bond interface)

To increase bandwidth or redundancy, it is possible to combine several physical interfaces into a bond interface (channel aggregation interface).

{% alert level="info" %}
Only network interfaces located on the same physical or virtual host can be combined.
{% endalert %}

Example configuring a bond interface:

1. Set custom labels on the interfaces intended for aggregation.

   Example of setting the `nni.example.com/bond-group=bond0` label on interfaces:

   ```shell
   d8 k label nni node-0-nic-fa163efbde48 nni.example.com/bond-group=bond0
   d8 k label nni node-0-nic-fa40asdxzx78 nni.example.com/bond-group=bond0
   ```

1. Prepare the configuration for creating the interface and apply it.

   Configuration example:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: NodeNetworkInterface
   metadata:
     name: nni-worker-01-bond0
   spec:
     nodeName: worker-01
     type: Bond
     heritage: Manual
     bond:
       bondName: bond0
       memberNetworkInterfaces:
         - labelSelector:
             matchLabels:
               # This is a service label that needs to be combined with the Bond interface on a specific node.
               network.deckhouse.io/node-name: worker-01
               # Custom label (was added to the interfaces in the previous step).
               nni.example.com/bond-group: bond0
   ```

1. Check the status of the created Bond interface:

   Get a list of interfaces:

   ```shell
   d8 k get nni
   ```

   Example output:

   ```console
   NAME                                                          MANAGEDBY   NODE                             TYPE     IFNAME      IFINDEX   STATE   AGE
   nni-worker-01-bond0                                           Manual      worker-01-b23d3a26-5fb4b-5s9fp   Bond     bond0       76        Up      7m48s
   ...
   ```

   Check the status of the desired interface:

   ```shell
   d8 k get nni nni-worker-01-bond0 -o yaml
   ```

   Example of interface status:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: NodeNetworkInterface
   metadata:
   ...
   status:
     conditions:
     - lastProbeTime: "2025-09-30T09:00:54Z"
       lastTransitionTime: "2025-09-30T09:00:39Z"
       message: Interface created
       reason: Created
       status: "True"
       type: Exists
     - lastProbeTime: "2025-09-30T09:00:54Z"
       lastTransitionTime: "2025-09-30T09:00:39Z"
       message: Interface is up and ready to send packets
       reason: Up
       status: "True"
       type: Operational
     deviceMAC: 6a:c7:ab:2a:a6:1e
     groupedLinks:
     - deviceMAC: fa:16:3e:92:14:40
       type: NIC
     ifIndex: 76
     ifName: bond0
     managedBy: Manual
     operationalState: Up
     permanentMAC: ""

   ```

## Configuring and connecting additional virtual networks for use in application pods

The Deckhouse Kubernetes Platform provides the ability to declaratively manage additional networks for application workloads (pods, virtual machines). At the same time:

* Each additional network implies a single L2 data exchange domain.
* Within the Pod’s network namespace, an additional network is represented as a tap interface.
* The following modes are currently available for L2 network implementation:
  * **Tagged VLAN**: Communication between Pods on different Nodes uses VLAN-tagged packets and the infrastructure’s network equipment for switching. This method allows to create up to 4096 additional networks within a single cluster.
  * **Direct access to a Node’s network interface**: Communication between Pods on different Nodes uses the local network interfaces of the Nodes.
* From a network management perspective, there are two types of networks:
  * **[Cluster network](#creating-a-publicly-accessible-network-cluster)**: A network available in all projects, under administrator management. Example: a public WAN network or a shared network for cross-project traffic.
  * **[Project network](#creating-a-project-network-user-network)**: A network available within a Namespace, under user management.

Custom resources [ClusterNetwork](/modules/sdn/cr.html#clusternetwork), [Network](/modules/sdn/cr.html#network), and [NetworkClass](/modules/sdn/cr.html#networkclass) are used to configure and connect additional networks for application pods.

{% alert level="info" %}
If the VLAN type was specified in the [Network](/modules/sdn/cr.html#network) or [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) resources, [NodeNetworkInterface](/modules/sdn/stable/cr.html#nodenetworkinterface) will also be created for VLAN and Bridge.
{% endalert %}

{% alert level="warning" %}
Before creating an additional network, [mark the interfaces](#marking-interfaces-for-organizing-additional-software-defined-networks) that will be used by it.
{% endalert %}

### Creating a publicly accessible network (cluster)

A custom resource [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) is used to create publicly accessible networks across the entire cluster.

#### Creating a network based on tagged traffic

To create a network based on tagged traffic, follow these steps:

1. Create and apply the [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) resource:

   In the `spec.type` parameter, specify the value `VLAN`. Tagged interfaces will be configured on the corresponding network interfaces of the nodes to ensure connectivity via the VLAN provided by the infrastructure.

   Example of a ClusterNetwork manifest for creating a public network based on tagged traffic:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterNetwork
   metadata:
     name: my-cluster-network
   spec:
     type: VLAN
     vlan:
       id: 900
     parentNodeNetworkInterfaces:
       labelSelector:
         matchLabels:
           # Manually applied label on NodeNetworkInterface resources.
           nic-group: extra
   ```

1. Check the status of the created resource with the command:

   ```shell
   d8 k get clusternetworks.network.deckhouse.io my-cluster-network -o yaml
   ```

    Example of the status of a ClusterNetwork resource:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: ClusterNetwork
   metadata:
   ...
   status:
     bridgeName: d8-br-900
     conditions:
     - lastTransitionTime: "2025-09-29T14:39:20Z"
       message: All node interface attachments are ready
       reason: AllNodeInterfaceAttachmentsAreReady
       status: "True"
       type: AllNodeAttachementsAreReady
     - lastTransitionTime: "2025-09-29T14:39:20Z"
       message: Network is operational
       reason: NetworkReady
       status: "True"
       type: Ready
     nodeAttachementsCount: 1
     observedGeneration: 1
     readyNodeAttachementsCount: 1

   ```

1. Check [the connection of the additional network to the interfaces on the nodes](#checking-the-connection-of-an-additional-network-to-interfaces-on-nodes).

#### Creating a network based on direct interface access

To create a network based on direct interface access, use the [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) resource. In the `spec.type` parameter, specify the value `Access`.  The corresponding network adapters on the nodes will be used directly to provide connectivity.

Example manifest for a network based on direct interface access:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: ClusterNetwork
metadata:
  name: my-cluster-network
spec:
  type: Access
  parentNodeNetworkInterfaces:
    labelSelector:
      matchLabels:
        # Manually applied label on NodeNetworkInterface resources.
        nic-group: extra
```

After creating the network, check [the connection of the additional network to the interfaces on the nodes](#checking-the-connection-of-an-additional-network-to-interfaces-on-nodes).

### Creating a project network (user network)

In order for users to be able to create their own dedicated networks based on tagged traffic, it is necessary to first describe the range of tags available to them and define the network interfaces on which they can be configured.
To do this, use the custom resource [NetworkClass](/modules/sdn/cr.html#clusternetworkclass).

Example:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: NetworkClass
metadata:
  name: my-network-class
spec:
  vlan:
    idPool:
    - 600-800
    - 1200
    parentNodeNetworkInterfaces:
      labelSelector:
        matchLabels:
          nic-group: extra
```

Upon user request, the administrator provides them with the name of the created NetworkClass, which is used when creating the project network.

An example of creating a custom network using the NetworkClass resource administrator is described in the section ["Creating a project network (user network)"](../../../../user/network/sdn/dedicated-networks.html).

### Checking the connection of an additional network to interfaces on nodes

After creating ClusterNetwork or Network, the controller will create a NodeNetworkInterfaceAttachment tracking resource to link it to a [NodeNetworkInterface](/modules/sdn/stable/cr.html#nodenetworkinterface).

To obtain a list of NodeNetworkInterfaceAttachment resources and information about a specific resource, use the following commands:

```shell
d8 k get nnia
d8 k get nnia my-cluster-network-... -o yaml
```

Example NodeNetworkInterfaceAttachment resource:

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: NodeNetworkInterfaceAttachment
metadata:
...
  finalizers:
    - network.deckhouse.io/nni-network-interface-attachment
    - network.deckhouse.io/pod-network-interface-attachment
  generation: 1
  name: my-cluster-network-...
...
spec:
  networkRef:
    kind: ClusterNetwork
    name: my-cluster-network
  parentNetworkInterfaceRef:
    name: right-worker-b23d3a26-5fb4b-h2bkv-nic-fa163eebea7b
  type: VLAN
status:
  bridgeNodeNetworkInterfaceName: right-worker-b23d3a26-5fb4b-h2bkv-bridge-900
  conditions:
    - lastTransitionTime: "2025-09-29T14:39:06Z"
      message: Vlan created
      reason: VLANCreated
      status: "True"
      type: Exist
    - lastTransitionTime: "2025-09-29T14:39:06Z"
      message: Bridged successfully
      reason: VLANBridged
      status: "True"
      type: Ready
  nodeName: right-worker-b23d3a26-5fb4b-h2bkv
  vlanNodeNetworkInterfaceName: right-worker-b23d3a26-5fb4b-h2bkv-vlan-900-60f3dc
```

The NodeNetworkInterfaceAttachment status will change to `True` immediately after the corresponding NodeNetworkInterface appears and transitions to the `Up` state.

To check the status of NodeNetworkInterface, use the command:

```shell
d8 k get nni
```

Example output:

```console
NAME                                                 MANAGEDBY   NODE                                TYPE     IFNAME      IFINDEX   STATE   AGE
...
right-worker-b23d3a26-5fb4b-h2bkv-bridge-900         Deckhouse   right-worker-b23d3a26-5fb4b-h2bkv   Bridge   d8-br-900   684       Up      14h
right-worker-b23d3a26-5fb4b-h2bkv-nic-fa163eebea7b   Deckhouse   right-worker-b23d3a26-5fb4b-h2bkv   NIC      ens3        2         Up      19d
right-worker-b23d3a26-5fb4b-h2bkv-vlan-900-60f3dc    Deckhouse   right-worker-b23d3a26-5fb4b-h2bkv   VLAN     ens3.900    683       Up      14h
...
```

## Configuring and connecting underlay networks for hardware device passthrough

This feature is designed for high-performance workloads that require direct access to hardware, such as DPDK applications.

### Key features

DKP implements the following features for working with underlay networks:

* **Hardware device passthrough**: Physical network interfaces (PF/VF) are directly exposed to pods, bypassing the kernel network stack for maximum performance.
* **SR-IOV configuration**: Automatic configuration of SR-IOV on selected Physical Functions to create Virtual Functions, allowing multiple pods to share the same hardware.
* **DPDK support**: Devices can be bound in different modes suitable for DPDK workloads:
  * **VFIO-PCI**: Explicitly connects a network device to the pod by binding it to the `vfio-pci` driver. The corresponding VFIO device files (e.g., `/dev/vfio/vfio0`) are mounted into the pod for userspace access.
  * **DPDK**: A universal mode that automatically selects the appropriate driver for the network adapter vendor. For Mellanox NICs, the device is bound to the `mlx5_core` driver with both the netdev interface and necessary device files mounted (InfiniBand verbs files, `/dev/net/tun`, and the corresponding sysfs directory). For other vendors, the device is bound via VFIO (same as VFIO-PCI mode).
  * **NetDev**: Only the Linux network interface is passed through to the pod as a standard kernel network device.

### Operation modes

The following device allocation modes are supported, which determine how physical interfaces are provided to hosts:

* [**Shared mode**](#creating-underlaynetwork-in-shared-mode): Creates Virtual Functions (VF) from Physical Functions (PF) using SR-IOV, allowing multiple pods to share the same hardware. Each pod receives one or more VFs.
* [**Dedicated mode**](#creating-underlaynetwork-in-dedicated-mode): Exposes each Physical Function as an exclusive device without SR-IOV. Each pod gets exclusive access to a complete PF, suitable for workloads requiring maximum performance.

### Automatic interface grouping

When [`autoBonding`](/modules/sdn/cr.html#underlaynetwork-v1alpha1-spec-autobonding) is enabled, the controller groups interfaces from multiple matched PFs into a single DRA device. The interfaces are passed through to the pod as separate network interfaces, allowing applications (e.g., DPDK) to handle bonding/aggregation at the application level. Note that this does not create kernel-level bonding interfaces inside the pod.

### Procedure for configuring and connecting physical interfaces to application subnets

To create underlay networks for forwarding hardware devices to pods, a custom resource [UnderlayNetwork](/modules/sdn/cr.html#underlaynetwork) is used. It provides direct connection of physical network interfaces (Physical Functions and Virtual Functions) to pods via Kubernetes Dynamic Resource Allocation (DRA).

#### Prerequisites for DPDK applications

Before configuring UnderlayNetwork resources, you must prepare the cluster's worker nodes for DPDK applications:

* Configure [hugepages](#configuring-hugepages).
* Configure [Topology Manager](#configuring-topology-manager).

##### Configuring hugepages

DPDK applications require hugepages for efficient memory management. Configure hugepages on all worker nodes using [NodeGroupConfiguration](/modules/node-manager/cr.html#nodegroupconfiguration):

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: NodeGroupConfiguration
metadata:
  name: hugepages-for-dpdk
spec:
  nodeGroups:
    - "*"  # Apply to all node groups.
  weight: 100
  content: |
    #!/bin/bash
    echo "vm.nr_hugepages = 4096" > /etc/sysctl.d/99-hugepages.conf
    sysctl -p /etc/sysctl.d/99-hugepages.conf
```

This configuration sets `vm.nr_hugepages = 4096` on all nodes, providing 8 GiB of hugepages (4096 pages × 2 MiB per page).

##### Configuring Topology Manager

For optimal performance, enable Topology Manager on NodeGroups of worker nodes where DPDK applications will run. This ensures that CPU, memory, and device resources are allocated from the same NUMA node.

Example NodeGroup configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: worker
spec:
  kubelet:
    topologyManager:
      enabled: true
      policy: SingleNumaNode
      scope: Container
  nodeType: Static
```

For more information, see:

* [topologyManager.enabled](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-enabled)
* [topologyManager.policy](/modules/node-manager/cr.html#nodegroup-v1-spec-kubelet-topologymanager-policy).

#### Preliminary steps before creating an UnderlayNetwork

Before creating an UnderlayNetwork, ensure that:

1. Physical network interfaces (NICs) are available on the nodes and are discovered as NodeNetworkInterface resources.
1. The interfaces you plan to use are Physical Functions (PF), not Virtual Functions (VF).
1. For [Shared mode](#operation-modes), the NICs must support SR-IOV.

##### Checking and configuring network interfaces (NodeNetworkInterface)

First, check which Physical Functions are available on your nodes:

```shell
d8 k get nni -l network.deckhouse.io/nic-pci-type=PF
```

Example output:

```console
NAME                            MANAGEDBY   NODE           TYPE   IFNAME   IFINDEX   STATE   VF/PF   Binding   Driver      Vendor   AGE
worker-01-nic-0000:17:00.0      Deckhouse   worker-01     NIC    ens3f0   3         Up      PF      NetDev    ixgbe       Intel    35d
worker-01-nic-0000:17:00.1      Deckhouse   worker-01     NIC    ens3f1   4         Up      PF      NetDev    ixgbe       Intel    35d
worker-02-nic-0000:17:00.0      Deckhouse   worker-02     NIC    ens3f0   3         Up      PF      NetDev    ixgbe       Intel    35d
worker-02-nic-0000:17:00.1      Deckhouse   worker-02     NIC    ens3f1   4         Up      PF      NetDev    ixgbe       Intel    35d
```

Label the interfaces that will be used for UnderlayNetwork:

```shell
d8 k label nni worker-01-nic-0000:17:00.0 nic-group=dpdk
d8 k label nni worker-01-nic-0000:17:00.1 nic-group=dpdk
d8 k label nni worker-02-nic-0000:17:00.0 nic-group=dpdk
d8 k label nni worker-02-nic-0000:17:00.1 nic-group=dpdk
```

{% alert level="info" %}
You can check the PCI information and SR-IOV support status for each interface:

```shell
d8 k get nni worker-01-nic-0000:17:00.0 -o json | jq '.status.nic.pci.pf'
```

The `status.nic.pci.pf.sriov.supported` section contains information about SR-IOV support.
{% endalert %}

#### Creating UnderlayNetwork in Dedicated mode

In Dedicated mode, each Physical Function is exposed as an exclusive device. This mode is suitable when:

* SR-IOV is not available or not needed.
* Each pod needs exclusive access to a complete PF.

To create an Underlay network in Dedicated mode, follow these steps:

1. Create and apply the UnderlayNetwork resource. In the `spec.mode` field, specify the value `Dedicated`.

   Example configuration:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-dedicated-network
   spec:
     mode: Dedicated
     autoBonding: false
     memberNodeNetworkInterfaces:
       - labelSelector:
           matchLabels:
             nic-group: dpdk # Label used to mark interfaces during the verification and configuration of network interfaces.
   ```

   When `autoBonding` is set to `true`, all matched PFs on a node are grouped into a single DRA device, exposing all PFs to the pod as separate interfaces. When `false`, each PF is published as a separate DRA device.

1. Check the status of the created UnderlayNetwork:

   ```shell
   d8 k get underlaynetwork dpdk-dedicated-network -o yaml
   ```

   Example status of UnderlayNetwork in `Dedicated` mode:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-dedicated-network
   ...
   status:
     observedGeneration: 1
     conditions:
     - message: All 2 member node network interface selectors have matches
       observedGeneration: 1
       reason: AllInterfacesAvailable
       status: "True"
       type: InterfacesAvailable
   ```

#### Creating UnderlayNetwork in Shared mode

In `Shared` mode, Virtual Functions (VF) are created from Physical Functions (PF) using SR-IOV, allowing multiple pods to share the same hardware. This mode requires SR-IOV support on the NICs.

To create an Underlay network in `Shared` mode, follow these steps:

1. Create and apply the UnderlayNetwork resource. In the `spec.mode` field, specify the value `Shared`.

   Example configuration:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-shared-network
   spec:
     mode: Shared
     autoBonding: true
     memberNodeNetworkInterfaces:
       - labelSelector:
           matchLabels:
             nic-group: dpdk
     shared:
       sriov:
         enabled: true
         numVFs: 8
   ```

   In this example:

   * `mode: Shared` enables SR-IOV and VF creation.
   * `autoBonding: true` groups one VF from each matched PF into a single DRA device.
   * `shared.sriov.enabled: true` enables SR-IOV on selected PFs.
   * `shared.sriov.numVFs: 8` creates 8 Virtual Functions per Physical Function.

   > The `mode` and `autoBonding` fields are immutable once set. Plan your configuration carefully before creating the resource.

1. After creating the UnderlayNetwork, monitor the SR-IOV configuration status:

   ```shell
   d8 k get underlaynetwork dpdk-shared-network -o yaml
   ```

   Example status of UnderlayNetwork in `Shared` mode:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: UnderlayNetwork
   metadata:
     name: dpdk-shared-network
   ...
   status:
     observedGeneration: 1
     sriov:
       supportedNICs: 4
       enabledNICs: 4
     conditions:
     - lastTransitionTime: "2025-01-15T10:30:00Z"
       message: SR-IOV configured on 4 NICs
       reason: SRIOVConfigured
       status: "True"
       type: SRIOVConfigured
     - lastTransitionTime: "2025-01-15T10:30:05Z"
       message: Interfaces are available for allocation
       reason: InterfacesAvailable
       status: "True"
       type: InterfacesAvailable
   ```

1. Verify that VFs have been created by checking NodeNetworkInterface resources:

   ```shell
   d8 k get nni -l network.deckhouse.io/nic-pci-type=VF
   ```

### Preparing namespaces for UnderlayNetwork usage

Before users can request UnderlayNetwork devices in their pods, the namespace must be labeled to enable UnderlayNetwork support. This is an administrative task that should be done for namespaces where DPDK applications will run:

```shell
d8 k label namespace mydpdk direct-nic-access.network.deckhouse.io/enabled=""
```
