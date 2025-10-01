---
title: "Ensuring high availability and fault tolerance (keepalived)"
permalink: en/admin/configuration/network/ingress/keepalived.html
description: "Configure keepalived for high availability in Deckhouse Kubernetes Platform. Failover configuration, and network redundancy setup for cluster infrastructure."
---

In Deckhouse Kubernetes Platform,
the [`keepalived`](/modules/keepalived/) module can be used to provide high availability and fault tolerance.

To configure keepalived clusters, custom resources are used.

## Module usage examples

### Multiple public IP addresses

In the following example, there are three public IP addresses on three front nodes.
Each virtual IP address is assigned to a separate VRRP group.
Thus, each address "jumps" independently of the others,
and if there are three nodes in the cluster with the `node-role.deckhouse.io/frontend: ""` labels,
each IP address gets its own master node.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front
spec:
  nodeSelector: # Required.
    node-role.deckhouse.io/frontend: ""
  tolerations:  # Optional.
  - key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
  vrrpInstances:
  - id: 1 # The unique cluster ID.
    interface:
      detectionStrategy: DefaultRoute # The card with the default route is used as a service network one.
    virtualIPAddresses:
    - address: 42.43.44.101/32
      # The interface parameter is omitted since IP addresses are based on the cards that service VRRP traffic.
  - id: 2
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 42.43.44.102/32
  - id: 3
    interface:
      detectionStrategy: DefaultRoute
    virtualIPAddresses:
    - address: 42.43.44.103/32
```

In the following example, there is a gateway with a pair of IP addresses for LAN and WAN.
In the case of the gateway, the private and public IPs are bind together, and they will "jump" between the nodes in tandem.
In the below example, the VRRP traffic is routed through the LAN interface.
It can be detected using the NetworkAddress method (assuming that each node has an IP address belonging to this subnet).

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: mygateway
spec:
  nodeSelector:
    node-role.deckhouse.io/mygateway: ""
  tolerations:
  - key: node-role.deckhouse.io/mygateway
    operator: Exists
  vrrpInstances:
  - id: 4 # Since "1", "2", "3" IDs are used in the "front" KeepalivedInstance above.
    interface:
      detectionStrategy: NetworkAddress
      networkAddress: 192.168.42.0/24
    virtualIPAddresses:
    - address: 192.168.42.1/24
      # In this case, we have already detected the local network (above); thus, the interface parameter can be safely omitted.
    - address: 42.43.44.1/28
      interface:
        detectionStrategy: Name
        name: ens7 # The interface for public IPs is called "ens7" on all nodes, therefore it needs to be named explicitly.
```

## Switching keepalived manually

1. Go to the target pod:

   ```shell
   d8 k -n d8-keepalived exec -it keepalived-<name> -- sh
   ```

1. Edit the `/etc/keepalived/keepalived.conf` file and in the line with the `priority` parameter,
   replace the value with the number of keepalived pods + 1.

1. Send a signal to reread the configuration:

   ```shell
   kill -HUP 1
   ```
