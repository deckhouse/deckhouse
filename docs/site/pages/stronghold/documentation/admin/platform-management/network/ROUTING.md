---
title: "Routing management"
permalink: en/stronghold/documentation/admin/platform-management/network/routing.html
lang: en
---

{% alert level="warning" %}
This feature is available only in Enterprise Edition.
{% endalert %}

To control static routes and IP rules on cluster nodes, use the static-routing-manager module.

To enable the module with default settings, apply the following `ModuleConfig` resource:

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

### Routing table

The `RoutingTable` resource describes the target routing table and its associated routes.

To create a route in the main routing table, do the following:

1. Apply the `RoutingTable` resource to create a new route (`10.0.0.0/8 via 192.168.0.1`) on nodes that match the specified nodeSelector:

   ```yaml
   d8 k apply -f - <<EOF
   apiVersion: network.deckhouse.io/v1alpha1
   kind: RoutingTable
   metadata:
   name: myrt-main
   spec:
     ipRoutingTableID: 254 # Main routing table ID is 254
     routes:
     - destination: 10.0.0.0/8
       gateway: 192.168.0.1
     nodeSelector:
       node-role.deckhouse.io: load-balancer
   EOF
   ```

1. To check the new route created in the main routing table, run the following command:

   ```shell
   ip -4 route ls
   ```

   In the output, you will see a list of routes, including the newly created `10.0.0.0/8 via 192.168.0.1`:

   ```console
   ...
   10.0.0.0/8 via 192.168.0.1 dev eth0 realm 216
   ...
   # The routed instruction 'realm 216' is used as a marker to identify the route managed by the module (d8 hex = 216 dec)
   ```

To create a route in an additional table, do the following:

1. Apply the `RoutingTable` resource to create a new route (`0.0.0.0/0 via 192.168.0.1`) in table 10000 on nodes that match the specified nodeSelector:

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
     ipRoutingTableID: 10000 # If spec.ipRoutingTableID isn't specified, it will be generated and placed into status automatically
       ...
   EOF
   ```

1. To check the new route created in the additional table, run the following command:

   ```shell
   ip -4 route ls table 10000
   ```

   In the output, you will see a list of routes from table 10000, including the newly created `default via 192.168.0.1`:

   ```console
   ...
   default via 192.168.0.1 dev eth0 realm 216
   ...
   ```

## Routing rules

The `IPRuleSet` resource describes a set of IP rules that will be created on the nodes with the corresponding labels.

To apply a rule, do the following:

1. Create the IPRuleSet resource to create an IP rule on nodes that match the specified nodeSelector:

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

1. To ensure the newly created rule was applied, run the following command:

   ```shell
   ip rule list
   ```

   In the output, you will see a list of configured rules:

   ```console
   ...
   50: from 192.168.111.0/24 to 172.16.8.0/21 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   50: from 192.168.222.0/24 to 8.8.8.8 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   50: from 192.168.222.0/24 to 172.16.8.0/21 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   50: from 192.168.111.0/24 to 8.8.8.8 ipproto tcp sport 100-200 dport 300-400 lookup 10000 realms 216
   ...
   ```
