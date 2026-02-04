---
title: "Additional networks"
permalink: en/admin/configuration/network/sdn/additional-networks.html
description: |
  Software Defined Networking: additional networks
---

The Deckhouse Kubernetes Platform provides the ability to declaratively manage additional networks for application workloads (pods, virtual machines). At the same time:

* Each additional network implies a single L2 data exchange domain.
* Within the Pod’s network namespace, an additional network is represented as a tap interface.
* The following modes are currently available for L2 network implementation:
  * Tagged VLAN: Communication between Pods on different Nodes uses VLAN-tagged packets and the infrastructure’s network equipment for switching. This method allows to create up to 4096 additional networks within a single cluster.
  * Direct access to a Node’s network interface: Communication between Pods on different Nodes uses the local network interfaces of the Nodes.
* From a network management perspective, there are two types of networks:
  * [Cluster network](#example-of-creating-a-cluster-network): A network available in all projects, under administrator management. Example: a public WAN network or a shared network for cross-project traffic.
  * [Project network](#creating-a-project-network-user-network): A network available within a Namespace, under user management.

## Configuring and connecting additional virtual networks for use in application pods

Custom resources [ClusterNetwork](/modules/sdn/cr.html#clusternetwork), [Network](/modules/sdn/cr.html#network), and [NetworkClass](/modules/sdn/cr.html#networkclass) are used to configure and connect additional networks for application pods.

{% alert level="info" %}
When you create a Network or ClusterNetwork resource with a VLAN type, the system first picks up the VLAN interface and connects it to the Bridge.
{% endalert %}

### Example of creating a cluster network

A custom resource [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) is used to create publicly accessible networks across the entire cluster.

#### Creating a network based on tagged traffic

To create a network based on tagged traffic, follow these steps:

1. Create and apply the [ClusterNetwork](/modules/sdn/cr.html#clusternetwork) resource:

   In the `spec.type` parameter, specify the value `VLAN`. Tagged interfaces will be configured on the corresponding network interfaces of the nodes to ensure connectivity via the VLAN provided by the infrastructure.

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
           nic-group: extra # Manually applied label on NodeNetworkInterface resources.
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

After creating ClusterNetwork, the controller will create a NodeNetworkInterfaceAttachment tracking resource to link it to a NodeNetworkInterface.

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
        nic-group: extra # Manually applied label on NodeNetworkInterface resources.
```

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

An example of creating a custom network using the NetworkClass resource administrator is described in the section ["Creating a network for a specific project"](../../../../user/network/sdn/dedicated-network-creating.html).
