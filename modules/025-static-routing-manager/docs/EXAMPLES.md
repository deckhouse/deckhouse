---
title: "The static-routing-manager module: examples"
---

## Creating a route in main routing table

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: myrt-main
spec:
  ipRouteTableID: 254 # main routing table id is 254
  routes:
  - destination: 10.0.0.0/8
    gateway: 192.168.0.1
  nodeSelector:
    node-role.deckhouse.io: load-balancer
```

According to this resource, the route `10.0.0.0.0/8 via 192.168.0.1` will be created on the nodes hitting `nodeSelector`:

```shell
$ ip -4 route ls
...
10.0.0.0/8 via 192.168.0.1 dev eth0 realm 216
...
```
The `realm 216` instruction in the route is used as a marker to identify the route under module control (d8 hex = 216 dec).

## Создание маршрута в дополнительной таблице

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: myrt-extra
spec:
  routes:
  - destination: 0.0.0.0/0
    gateway: 192.168.0.1
  nodeSelector:
    node-role.deckhouse.io: load-balancer
status:
  ...
  ipRouteTableID: 10000 # if spec.ipRouteTableID is not specified, it will be generated automatically and placed in status
  ...
```

According to this resource, the route `10.0.0.0.0/8 via 192.168.0.1` will be created on the nodes hitting `nodeSelector` in the table 10000:

```shell
$ ip -4 route ls table 10000
default via 192.168.0.1 dev eth0 realm 216
```

## Настройка `ip rule`

```yaml

```

According to this resource, an ip rule will be created on the nodes falling under `nodeSelector`:

```shell

```
