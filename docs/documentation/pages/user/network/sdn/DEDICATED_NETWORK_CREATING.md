---
title: "Creating a network for a specific project"
permalink: en/user/network/sdn/dedicated-network-creating.html
---

To create a network for a specific project, use the [Network](cr.html#network) and [NetworkClass](cr.html#networkclass) resources provided to you by the administrator:

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
