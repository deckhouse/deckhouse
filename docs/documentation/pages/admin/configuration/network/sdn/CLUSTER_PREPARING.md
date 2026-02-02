---
title: "Preparing the cluster for SDN use"
permalink: en/admin/configuration/network/sdn/cluster-preparing.html
description: |
  Preparing the cluster for use with software defined networking
---

Software-defined networking (SDN) functions within the Deckhouse Kubernetes Platform are implemented using the [`sdn`](/modules/sdn/) module. DKP supports the following software-defined networking capabilities:

* [Node network interfaces configuration](node-network-configuration-management.html).
* [Additional networks](additional-networks.html).
* [Underlay networks for hardware device passthrough](underlay-networks.html).

## Preparing the infrastructure for module use

Before using software-defined networks in a cluster, preliminary infrastructure preparation is required:

* **For creating additional networks based on tagged VLANs:**
  * Allocate VLAN ID ranges on the data center switches and configure them on the corresponding switch interfaces.
  * Select physical interfaces on the nodes for subsequent configuration of tagged VLAN interfaces. You can reuse interfaces already used by the DKP local network.

* **For creating additional networks based on direct, untagged access to a network interface:**
  * Reserve separate physical interfaces on the nodes and connect them into a single local network at the data center level.

## Actions after enabling the sdn module

After enabling the module, NodeNetworkInterface resources will automatically appear in the cluster, reflecting the current state of the nodes.

To check for resources, use the command:

```shell
d8 k get nodenetworkinterface
```

> The `nodenetworkinterface` resource can  in commands be abbreviated to `nni`.

Example output:

```console
NAME                            MANAGEDBY   NODE           TYPE     IFNAME           IFINDEX   STATE      AGE
virtlab-ap-0-nic-1c61b4a68c2a   Deckhouse   virtlab-ap-0   NIC      eth1             3         Up         35d
virtlab-ap-0-nic-fc34970f5d1f   Deckhouse   virtlab-ap-0   NIC      eth0             2         Up         35d
virtlab-ap-1-nic-1c61b4a6a0e7   Deckhouse   virtlab-ap-1   NIC      eth1             3         Up         35d
virtlab-ap-1-nic-fc34970f5c8e   Deckhouse   virtlab-ap-1   NIC      eth0             2         Up         35d
virtlab-ap-2-nic-1c61b4a6800c   Deckhouse   virtlab-ap-2   NIC      eth1             3         Up         35d
virtlab-ap-2-nic-fc34970e7ddb   Deckhouse   virtlab-ap-2   NIC      eth0             2         Up         35d
```

{% alert level="info" %}
When discovering node interfaces, the controller affixes the following labels, which are service labels:

```yaml
    labels:
      network.deckhouse.io/interface-mac-address: fa163eebea7b
      network.deckhouse.io/interface-type: VLAN
      network.deckhouse.io/vlan-id: 900
      network.deckhouse.io/node-name: worker-01
    annotations:
      network.deckhouse.io/heritage: NetworkController
```

{% endalert %}

In this example, each cluster node has two network interfaces: eth0 (DKP local network) and eth1 (dedicated interface for additional networks).

Next, you need to label the reserved interfaces with an appropriate tag for additional networks:

```shell
d8 k label nodenetworkinterface virtlab-ap-0-nic-1c61b4a68c2a nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-1-nic-1c61b4a6a0e7 nic-group=extra
d8 k label nodenetworkinterface virtlab-ap-2-nic-1c61b4a6800c nic-group=extra
```

To increase bandwidth or provide redundancy, it is also possible to combine several physical interfaces into a Bond. For more details, see the section [Creating a Bond Interface](node-network-configuration-management.html#example-of-creating-a-bond-interface).
