---
title: "The keepalived module: usage"
---

## Three public IP addresses

Suppose there are three public IP addresses on three front nodes. Each virtual IP address is placed in a separate VRRP group. Thus, each address "jumps" independently of the others, and if there are three nodes in the cluster with the `node-role.deckhouse.io/frontend: ""` labels, then each IP gets its own MASTER node.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front
spec:
  nodeSelector: # mandatory
    node-role.deckhouse.io/frontend: ""
  tolerations:  # optional
  - key: dedicated.deckhouse.io
    operator: Equal
    value: frontend
  vrrpInstances:
  - id: 1 # the unique cluster ID
    interface:
      detectionStrategy: DefaultRoute # the card with the default route is used as a service network one
    virtualIPAddresses:
    - address: 42.43.44.101/32
      # the interface parameter is omitted since IP addresses are based on the cards that service VRRP traffic
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

Suppose there is a gateway with a pair of IP addresses for LAN and WAN. In the case of the gateway, the private and public IPs are bind together, and they will "jump" between the nodes in tandem. In the below example, the VRRP traffic is routed through the LAN interface. It can be detected using the NetworkAddress method (assuming that each node has an IP belonging to this subnet).

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
  - id: 4 # since "1", "2", "3" IDs are used in the "front" KeepalivedInstance above
    interface:
      detectionStrategy: NetworkAddress
      networkAddress: 192.168.42.0/24
    virtualIPAddresses:
    - address: 192.168.42.1/24
      # in this case, we have already detected the local network (above); thus, the interface parameter can be safely omitted
    - address: 42.43.44.1/28
      interface:
        detectionStrategy: Name
        name: ens7 # we use the fact that an interface for public IPs is called "ens7" on all nodes
```