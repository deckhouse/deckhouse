---
title: "Additional networks for use in application pods"
permalink: en/user/network/sdn/dedicated-networks.html
---

DKP implements the ability to use additional software-defined networks (hereinafter referred to as additional networks) for application workloads (pods, virtual machines). You can use the following types of networks:

- Cluster (public) — a network that is publicly available in each project, configured and managed by the administrator. An example is a public WAN network or a shared network for traffic exchange between projects. To create such a network and use it for application pods, contact the cluster administrator.
- Project network (user network) — a network accessible within a namespace, created and managed by the user using the NetworkClass resource provided by the administrator.

For more information about additional software-defined networks, see [Configuring and connecting additional virtual networks for use in application pods](../../../admin/configuration/network/sdn/cluster-preparing-and-sdn-enabling.html#configuring-and-connecting-additional-virtual-networks-for-use-in-application-pods).

## Creating a project network (user network)

To create a network for a specific project, use the [Network](/modules/sdn/cr.html#network) and [NetworkClass](/modules/sdn/cr.html#networkclass) resources provided to you by the administrator:

1. Create and apply the Network resource by specifying the name of the NetworkClass resource obtained from the administrator in the `spec.networkClass` field:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
     name: my-network
     namespace: my-namespace
   spec:
     networkClass: my-network-class # The name of the NetworkClass resource obtained from the administrator.
   ```

   > Static identification of the VLAN ID number from the pool assigned by the cluster or network administrator is supported. If the value of the `spec.vlan.id` field is not specified in the resource specification, the VLAN ID will be assigned dynamically.

1. After creating the Network resource you can check its status:

   ```shell
   d8 k -n my-namespace get network my-network -o yaml
   ```

   Example of the status of a Network resource:

   ```yaml
   apiVersion: network.deckhouse.io/v1alpha1
   kind: Network
   metadata:
   ...
   status:
     bridgeName: d8-br-600
     conditions:
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: All node interface attachments are ready
       reason: AllNodeInterfaceAttachmentsAreReady
       status: "True"
       type: AllNodeAttachementsAreReady
     - lastTransitionTime: "2025-09-29T14:51:26Z"
       message: Network is operational
       reason: NetworkReady
       status: "True"
       type: Ready
     nodeAttachementsCount: 1
     observedGeneration: 1
     readyNodeAttachementsCount: 1
     vlanID: 600
   ```

After creating a network, you can [connect it to pods](#connecting-additional-networks-to-pods).

## Connecting additional networks to pods

You can connect cluster networks and project networks to pods. To do this, use the pod annotation, specifying the parameters of the additional networks to be connected.

Example of adding two additional networks (cluster `my-cluster-network` and project network `my-network`) to a pod:

```yaml
network.deckhouse.io/networks-spec: |
  [
    {
      "type": "Network", # Connecting the my-network project network.
      "name": "my-network",
      "ifName": "veth_mynet",    # TAP interface name inside the pod (optional).
      "mac": "aa:bb:cc:dd:ee:ff" # MAC address to assign to the TAP interface (optional).
    },
    {
      "type": "ClusterNetwork", # Connecting to the public network my-cluster-network.
      "name": "my-cluster-network",
      "ifName": "veth_public",
    }
  ]
```
