---
title: "The keepalived module: examples"
---

## Three public IP addresses

There are three public IP addresses, each of which is linked to a separate front-end server. Each of the virtual IP addresses is part of a separate VRRP group, so each address switches independently of the others. If there are three nodes in the cluster with the label `none-role.deck house.io/frontend : ""`, then each IP will be linked to its main server.

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: KeepalivedInstance
metadata:
  name: front
spec:
  nodeSelector: # Mandatory.
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

There is a gateway with two IP addresses: one for the internal (LAN) and one for the external (WAN) network. These two IP addresses work in pairs and switch between nodes together. The internal interface (LAN) is used for VRRP service traffic (traffic used to manage the VRRP group). This interface is defined using the `Network Address` functions with the assumption that each node has an IP address from the same subnet.

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
        name: ens7 # we use the fact that an interface for public IPs is called "ens7" on all nodes
```
