---
title: Lightweight logs
permalink: en/admin/configuration/logging/lightweight.html
description: "Viewing current container logs in Deckhouse Kubernetes Platform without deploying Loki and log-shipper. A Loki-compatible API on top of log files on nodes, accessible through Grafana and a query builder."
---

Lightweight logs in Deckhouse Kubernetes Platform (DKP) let you view the current logs (the current state) of containers in projects and across the whole cluster without deploying a storage system or configuring log delivery. Lightweight logs are read directly from the CRI-created files on the node, so viewing them does not require enabling the [`loki`](/modules/loki/) and [`log-shipper`](/modules/log-shipper/) modules or allocating resources for them — the only extra cost is the disk space the logs already take up regardless of this mechanism.

Lightweight logs are exposed through a Loki-compatible HTTP API, so they are available in Grafana and the DKP web interface the same way as regular Loki-backed logs. You can retrieve lightweight logs using LogQL, not only with the `d8 k logs` command.

## Benefits of lightweight logs

Lightweight logs in DKP have the following benefits:

- No storage overhead. Unlike Loki, lightweight logs don't require a separate storage system. Logs are read in real time from files that already reside on the node's disk anyway (since they are created by the CRI).
- Loki-compatible API. Instead of running `d8 k logs` for one pod at a time, you can query the entire available set of logs at once using Grafana. You can build graphs and run selections and aggregations with LogQL.
- Flexible filtering in the UI. The DKP web interface lets you filter logs by various parameters.
- A single API across all log storage options. Lightweight logs are accessed through the same Loki-compatible interface as the other DKP logging tools.

{% alert level="info" %}
Because lightweight logs and Loki-based storage share the same Loki-compatible API, you can use them at the same time.
{% endalert %}

## Features and limitations

Lightweight logs in DKP have one important limitation: a pod's container logs stop being queryable as soon as the pod is removed from the cluster. There is no long-term storage of lightweight logs.

## Enabling and disabling

Lightweight logs are implemented using the `observability` module. If the module is enabled, they are enabled by default and require no additional configuration.

To disable lightweight logs, set the `lightweightLogs.enabled : false` parameter in the [`observability`](/modules/observability/) module configuration:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: observability
spec:
  version: 1
  enabled: true
  settings:
    lightweightLogs:
      enabled: false
```

To enable or disable lightweight logs in the DKP web interface, go to the `observability` module settings, select "Additional settings" → "Lightweight logs", and toggle the `Enabled` parameter.

## Viewing logs

To view lightweight logs in the DKP web interface, go to "Monitoring" → "Data exploration" (at the cluster level or within a specific project). Select the `lightweight-logs` data source.

The result set includes the container logs of all pods running at query time in the selected namespace, or in all namespaces of the cluster (including system ones).

Viewing logs requires the user to have [the appropriate permissions](/modules/observability/permissions.html) (`resources: ["logs"]`).
