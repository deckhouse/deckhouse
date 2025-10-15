---
title: "Configuring network and cluster node monitoring"
permalink: en/virtualization-platform/documentation/admin/platform-management/monitoring/configuring/network-and-pods.html
---

## Network Monitoring

DVP can perform network monitoring between all cluster nodes, as well as between cluster nodes and external hosts. When monitoring is configured, each node sends ICMP packets twice per second to all other cluster nodes (and to optional external nodes) and exports data to the monitoring system.

The `monitoring-ping` module tracks any changes in the `.status.addresses` field of a node. If changes are detected, a hook is triggered that collects a complete list of node names and their addresses, and passes it to the DaemonSet, which recreates the pods. Thus, `ping` always checks an up-to-date list of nodes.

{% alert level="warning" %}
The `monitoring-ping` module must be enabled.
{% endalert %}

### Adding Additional IP Addresses for Monitoring

To add additional monitoring IP addresses, use the [`externalTargets`](/modules/monitoring-ping/configuration.html#parameters-externaltargets) parameter of the `monitoring-ping` module.

Example module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: monitoring-ping
spec:
  version: 1
  enabled: true
  settings:
    externalTargets:
    - name: google-primary
      host: 8.8.8.8
    - name: yaru
      host: ya.ru
    - host: youtube.com
```

> The `name` field is used in Grafana to display related data. If the `name` field is not specified, the required `host` field is used.

## Cluster Node Monitoring

To enable cluster node monitoring, you need to enable the `monitoring-kubernetes` module if it is not already enabled. You can enable cluster monitoring in the [Deckhouse web interface](/modules/console/stable/), or using the following command:

```shell
d8 platform module enable monitoring-kubernetes
```

Similarly, you can enable the `monitoring-kubernetes-control-plane` and `extended-monitoring` modules.
