---
title: "Configuring network interaction and cluster node monitoring"
permalink: en/admin/configuration/monitoring/configuring/network-and-nodes.html
---

## Network interaction monitoring

DKP can perform monitoring of network interaction between all cluster nodes, as well as between cluster nodes and external hosts. When monitoring is configured, each node sends ICMP packets twice per second to all other cluster nodes (and to optional external nodes) and exports data to the monitoring system.

Features of network interaction monitoring:

- automatically checks the availability of all cluster nodes (and, optionally, external systems) using ICMP (ping) — testing is started every two seconds;
- all results are exported in metrics format to the Prometheus monitoring system.;
- included is a ready—made dashboard for Grafana, where current availability, delay schedules, and potential network connectivity issues are visualized in real time (for more details, see the [Grafana](../../../../user/web/grafana.html) section.);
- allows you to quickly identify nodes with degraded connectivity and speeds up the response to incidents.

The [`monitoring-ping`](/modules/monitoring-ping/) module tracks any changes to the `.status.addresses` field of a node. If changes are detected, a hook is triggered that collects the complete list of node names and their addresses, and passes it to the DaemonSet, which recreates the pods. Thus, `ping` checks always the current list of nodes.

{% alert level="warning" %}
The [`monitoring-ping`](/modules/monitoring-ping/) module must be enabled.
{% endalert %}

### Adding additional IP addresses for monitoring

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

## Cluster node monitoring

To enable cluster node monitoring, you need to enable the [`monitoring-kubernetes`](/modules/monitoring-kubernetes/) module if it's not already enabled. You can enable cluster monitoring in the [Deckhouse web interface](/modules/console/), or using the following command:

```shell
d8 platform module enable monitoring-kubernetes
```

Similarly, you can enable the [`monitoring-kubernetes-control-plane`](/monitoring-kubernetes-control-plane/) and [`extended-monitoring`](/modules/extended-monitoring/) modules.
