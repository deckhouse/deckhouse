---
title: "Модуль static-routing-manager: примеры"
---

## Создание маршрута в основной таблице "main"

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: RoutingTable
metadata:
  name: myrt-main
spec:
  ipRoutingTableID: 254 # main routing table id is 254
  routes:
  - destination: 10.0.0.0/8
    gateway: 192.168.0.1
  nodeSelector:
    node-role.deckhouse.io: load-balancer
```

Согласно данному ресурсу на узлах, попадающих под `nodeSelector`, будет создан маршрут `10.0.0.0/8 via 192.168.0.1`:

```shell
$ ip -4 route ls
...
10.0.0.0/8 via 192.168.0.1 dev eth0 realm 216
...
```

Инструкция `realm 216` в маршруте используется как маркер для идентификации маршрута под управлением модуля (d8 hex = 216 dec).

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
  ipRoutingTableID: 10000 # если spec.ipRoutingTableID не указан, он будет сгенерирован автоматически и размещён в status
  ...
```

Согласно данному ресурсу на узлах, попадающих под `nodeSelector`, будет создан маршрут `0.0.0.0/0 via 192.168.0.1` в таблице `10000`:

```shell
$ ip -4 route ls table 10000
default via 192.168.0.1 dev eth0 realm 216
```

## Создание `ip rule`

```yaml
apiVersion: network.deckhouse.io/v1alpha1
kind: IPRuleSet
metadata:
  name: myiprule
spec:
  rules:
  - selectors:
      from:
      - 192.168.111.0/24
      - 192.168.222.0/24
      to:
      - 8.8.8.8/32
      - 172.16.8.0/21
      sportRange:
        start: 100
        end: 200
      dportRange:
        start: 300
        end: 400
      ipProto: 6
    actions:
      lookup:
        routingTableName: myrt-extra
    priority: 50
  nodeSelector:
    node-role.deckhouse.io: load-balancer

```

Согласно данному ресурсу на узлах, попадающих под `nodeSelector`, будет создан ip rule:

```shell
$ ip rule list
...
50: from 192.168.111.0/24 to 172.16.8.0/21 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
50: from 192.168.222.0/24 to 8.8.8.8 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
50: from 192.168.222.0/24 to 172.16.8.0/21 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
50: from 192.168.111.0/24 to 8.8.8.8 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
...

```
