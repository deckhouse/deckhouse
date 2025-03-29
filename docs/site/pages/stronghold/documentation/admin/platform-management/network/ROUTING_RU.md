---
title: "Управление маршрутизацией"
permalink: ru/stronghold/documentation/admin/platform-management/network/routing.html
lang: ru
---

{% alert level="warning" %}
Функция доступна только в Enterprise Edition.
{% endalert %}

Для управления статичными маршрутами и правилами IP-rule на узлах кластера, можно использовать возможности модуля static-routing manager.

Чтобы включить модуль static-routing-manager с настройками по умолчанию, примените следующий ресурс `ModuleConfig`:

```yaml
d8 k apply -f - <<EOF
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: static-routing-manager
spec:
  version: 1
  enabled: true
EOF
```

### Таблица маршрутизации

Ресурс `RoutingTable` описывает желаемую таблицу маршрутизации и содержащиеся в ней маршруты.

Чтобы создать маршрут в основной таблице маршрутизации main, выполните следующее:

1. Примените ресурс `RoutingTable`, согласно которому на узлах, подпадающих под nodeSelector, будет создан маршрут `10.0.0.0/8 via 192.168.0.1`:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: network.deckhouse.io/v1alpha1
   kind: RoutingTable
   metadata:
   name: myrt-main
   spec:
     ipRoutingTableID: 254 # ID основной таблицы маршрутизации – 254
     routes:
     - destination: 10.0.0.0/8
       gateway: 192.168.0.1
     nodeSelector:
       node-role.deckhouse.io: load-balancer
   EOF
   ```

1. Чтобы проверить созданный маршрут в основной таблице, выполните следующую команду:

   ```shell
   ip -4 route ls
   ```

   В результате будет выведен список маршрутов, включая созданный `10.0.0.0/8 via 192.168.0.1`:

   ```console
   ...
   10.0.0.0/8 via 192.168.0.1 dev eth0 realm 216
   ...
   # Инструкция realm 216 в маршруте используется как маркер для идентификации маршрута под управлением модуля (d8 hex = 216 dec).
   ```

Чтобы создать маршрут в дополнительной таблице, выполните следующее:

1. Примените ресурс `RoutingTable`, согласно которому на узлах, подпадающих под nodeSelector, будет создан маршрут `0.0.0.0/0 via 192.168.0.1` в таблице 10000:

   ```yaml
   d8 k apply -f - <<EOF
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
     ipRoutingTableID: 10000 # Если spec.ipRoutingTableID не указан, он будет сгенерирован автоматически и размещён в status
       ...
   EOF
   ```

1. Чтобы проверить созданный маршрут в дополнительной таблице, выполните следующую команду:

   ```shell
   ip -4 route ls table 10000
   ```

   В результате будет выведен список маршрутов из таблицы 10000, включая созданный `default via 192.168.0.1`:

   ```console
   ...
   default via 192.168.0.1 dev eth0 realm 216
   ...
   ```

## Правила маршрутизации

Ресурс `IPRuleSet` описывает набор правил (IP-rule), которые будут созданы на узлах с соответствующими метками.

Чтобы применить правило, выполните следующее:

1. Создайте ресурс IPRuleSet, согласно которому на узлах, подпадающих под nodeSelector, будет создан IP-rule:

   ```yaml
   d8 k apply -f - <<EOF
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
   EOF
   ```

1. Чтобы убедиться в том, что правило было применено, выполните следующую команду:

   ```shell
   ip rule list
   ```

   В результате будет выведен список настроенных правил:

   ```console
   ...
   50: from 192.168.111.0/24 to 172.16.8.0/21 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   50: from 192.168.222.0/24 to 8.8.8.8 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   50: from 192.168.222.0/24 to 172.16.8.0/21 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   50: from 192.168.111.0/24 to 8.8.8.8 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   ...
   ```
