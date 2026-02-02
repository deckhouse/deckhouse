---
title: "Node network interfaces configuration"
permalink: en/admin/configuration/network/sdn/node-network-configuration-management.html
description: |
  Software Defined Networking: node network interfaces configuration
---

A declarative API is used to configure network interfaces on nodes.

The following network interface configuration options are supported on nodes:

* port aggregation
* combining network interfaces into a bridge
* configuring VLAN interfaces.

## Example of creating a Bond interface

Combining multiple physical interfaces into a Bond is used to increase bandwidth or provide redundancy.

{% alert level="info" %}
A Bond interface can only be created between NIC interfaces that are located on the same physical or virtual host.
{% endalert %}

Example configuring Bond interface:

1. Set custom labels for interfaces that can be combined to create a Bond interface.

   > The `nodenetworkinterface` resource can  in commands be abbreviated to `nni`.

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
               network.deckhouse.io/node-name: worker-01 # This is a service label that needs to be combined with the Bond interface on a specific node.
               nni.example.com/bond-group: bond0 # Custom label, we need to set it ourselves on selected interfaces.
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
