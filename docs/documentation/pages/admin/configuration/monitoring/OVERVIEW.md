---
title: "Monitoring in Deckhouse Kubernetes Platform"
permalink: en/admin/configuration/monitoring/
description: "Configure comprehensive monitoring for Deckhouse Kubernetes Platform with Prometheus and Grafana. Metrics collection, alerting, dashboards, and SLA monitoring for cluster health."
---

Deckhouse Kubernetes Platform (DKP) provides a Kubernetes monitoring solution based on **Prometheus** and **Grafana**.
DKP automatically configures metrics collection in the cluster from nodes, pods, and key cluster components (etcd, kube-apiserver, CoreDNS), which enables preset dashboards for analyzing CPU, memory, disk, and network usage.

Cluster monitoring is enabled by default in the `Default` and `Managed` [module bundles](../#module-bundles).

All components, including Prometheus and Alertmanager, operate in a fault-tolerant mode and can be used in cloud environments and on bare-metal servers.

The principles of Prometheus operation is covered in [Configuring a system for collecting and storing metrics](./prometheus.html).

Several types of monitoring are implemented in DKP:

- [Hardware resource monitoring](#hardware-resource-monitoring)
- [Kubernetes monitoring](#kubernetes-monitoring)
- [Ingress monitoring](#ingress-monitoring)
- [Control plane monitoring](#control-plane-monitoring)
- [Network interaction monitoring](./configuring/network-and-nodes.html)
- [Extended monitoring](#extended-monitoring-mode)
- [Cluster SLA monitoring](#cluster-sla-monitoring)

DKP includes an [alerting system](#alerts) that supports sending event notifications, including to [external systems](#sending-alerts-to-external-systems).

## Hardware resource monitoring

Tracking of cluster hardware resource capacity is provided with graphs showing utilization of:

- CPU
- Memory
- Disk
- Network

Graphs are available with aggregation by:

- Pods
- Controllers
- Namespaces
- Nodes

## Kubernetes monitoring

The module [`monitoring-kubernetes`](/modules/monitoring-kubernetes/) is designed for basic cluster node monitoring.

It provides secure metrics collection and offers a basic set of rules for monitoring:

- Current container runtime version (docker, containerd) on the node and its compliance with versions allowed for use.
- Overall cluster monitoring subsystem health (Dead man's switch).
- Available file descriptors, sockets, free space, and inodes.
- Operation of `kube-state-metrics`, `node-exporter`, `kube-dns`.
- Cluster node state (NotReady, drain, cordon).
- Time synchronization state on nodes.
- Cases of prolonged CPU steal exceeding.
- Conntrack table state on nodes.
- Pods with incorrect state (as a possible consequence of kubelet issues) and more.

## Ingress monitoring

Statistics collection for [`ingress-nginx`](/modules/ingress-nginx/) in Prometheus is implemented with detailed metrics (response time, codes, geography, etc.), available in different dimensions (namespace, vhost, ingress). Data is visualized in Grafana with interactive dashboards.
Detailed description is available in the section about [Ingress monitoring](../network/ingress/alb/nginx.html#monitoring-and-statistics).

The module is enabled by default in the `Default` and `Managed` [module bundles](../#module-bundles).

### Disabling collection of detailed statistics from Ingress resources

By default, DKP collects detailed statistics from all Ingress resources in the cluster, which generates a high load on the monitoring system.

To disable statistics collection, add the label `ingress.deckhouse.io/discard-metrics: "true"` to the corresponding namespace or Ingress resource.

- Example of disabling statistics (metrics) collection for all Ingress resources in the `review-1` namespace:

  ```shell
  d8 k label ns review-1 ingress.deckhouse.io/discard-metrics=true
  ```

- Example of disabling statistics (metrics) collection for all `test-site` Ingress resources in the `development` namespace:

  ```shell
  d8 k label ingress test-site -n development ingress.deckhouse.io/discard-metrics=true
  ```

## Control plane monitoring

Control plane monitoring is performed using the [`monitoring-kubernetes-control-plane`](/modules/monitoring-kubernetes-control-plane/) module, which organizes secure metrics collection and provides a basic set of monitoring rules for the following cluster components:

- kube-apiserver
- kube-controller-manager
- kube-scheduler
- kube-etcd

## Cluster monitoring

DKP securely collects monitoring metrics and configures rules.

DKP monitoring capabilities:

- Monitoring current container runtime version (containerd) on the node and its compliance with versions allowed for use in DKP.
- Monitoring cluster monitoring subsystem health ("Dead man's switch").
- Monitoring available file descriptors, sockets, free space, and inodes.
- Monitoring cluster node state (NotReady, drain, cordon).
- Operation of `kube-state-metrics`, `node-exporter`, `kube-dns`.
- Monitoring time synchronization state on nodes.
- Monitoring cases of prolonged CPU steal exceeding.
- Monitoring Conntrack table state on nodes.
- Monitoring pods with incorrect state (as a possible consequence of kubelet issues).
- Monitoring control plane components (implemented by the `monitoring-kubernetes-control-plane` module).

## Extended monitoring mode

DKP supports an extended monitoring mode via the [`extended-monitoring`](/modules/extended-monitoring/) module, allowing you to configure:

- Monitoring secrets in the cluster (Secret objects) and TLS certificate expiration in them.
- Collecting Kubernetes cluster events as metrics.
- Monitoring container image availability in registry used by controllers (Deployments, StatefulSets, DaemonSets, CronJobs).
- Monitoring objects in namespaces that have the `extended-monitoring.deckhouse.io/enabled=""` label.

The module can send alerts based on the following metrics:

- Free space and inodes on node disks
- Node utilization
- Pod and container image availability
- Certificate expiration
- Other cluster events

## Alerts

Monitoring in DKP includes event notifications. The standard delivery includes a set of basic warnings covering cluster state and its components. There is also the ability to add custom alerts.

### Sending alerts to external systems

DKP supports sending alerts using Alertmanager:

- Via SMTP protocol
- To PagerDuty
- [To Slack](alerts-integrations.html#example-of-sending-alerts-to-slack-with-filter)
- [To Telegram](alerts-integrations.html#sending-alerts-to-telegram)
- Via Webhook
- Through any other channels supported in Alertmanager

Examples of DKP monitoring integration with external systems are available in [Configuring integrations](alerts-integrations.html).

## Cluster SLA monitoring

Availability assessment in DKP is performed by the [`upmeter`](/modules/upmeter/) module.

Composition of the `upmeter` module:

- **agent**: Runs on master nodes and performs availability probes, sends results to the server.
- **upmeter**: Collects results and maintains an API server for their retrieval.
- **front**:
  - **status**: Shows availability level for the last 10 minutes (requires authorization, but it can be disabled).
  - **webui**: Shows a dashboard with statistics on probes and availability groups (requires authorization).
- **smoke-mini**: Maintains continuous *smoke testing* using StatefulSet.

The module sends about 100 metric readings every 5 minutes. This value depends on the number of enabled Deckhouse Kubernetes Platform modules.
