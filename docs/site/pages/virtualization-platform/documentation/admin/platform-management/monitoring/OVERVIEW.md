---
title: "Monitoring in Deckhouse Virtualization Platform"
permalink: en/virtualization-platform/documentation/admin/platform-management/monitoring/
---

Deckhouse Virtualization Platform (DVP) provides a Kubernetes monitoring solution based on **Prometheus** and **Grafana**.
The module automatically configures metrics collection from nodes, pods, and key cluster components (etcd, kube-apiserver, CoreDNS), and offers pre-installed dashboards for analyzing CPU, memory, disk, and network usage.

All components operate in a fault-tolerant mode, including Prometheus and Alertmanager, and are adapted for clouds and bare metal.

The principles of operation and configuration of Prometheus are covered in the [article](docs/documentation/pages/admin/monitoring/prometheus.html).

Several types of monitoring are implemented:
- [Hardware resource monitoring](#hardware-resource-monitoring).
- [Kubernetes monitoring](#kubernetes-monitoring).
- [Ingress monitoring](#ingress-monitoring).

[Extended monitoring mode](#extended-monitoring-mode) and [alerting](#alerts) are provided, including [to external systems](#sending-alerts-to-external-systems). Monitoring via dashboards is available, as well as cluster SLA monitoring.

## Hardware resource monitoring

Tracking of cluster hardware resource utilization with graphs for:

- `CPU`: CPU utilization monitoring.
- `Memory`: Memory usage tracking.
- `Disk`: Disk space and I/O monitoring.
- `Network`: Network traffic and connectivity monitoring.

Graphs are available with aggregation:

- `By pods`: Pod-level resource utilization.
- `By controllers`: Controller-level metrics aggregation.
- `By namespaces`: Namespace-based resource grouping.
- `By nodes`: Node-level hardware resource monitoring.

## Kubernetes monitoring

The module is designed for basic cluster node monitoring.

It provides secure metrics collection and offers a basic set of monitoring rules for:
- `Container runtime version`: Current container runtime version (docker, containerd) on the node and its compliance with versions allowed for use.
- `Cluster monitoring health`: Overall health of the cluster monitoring subsystem (Dead man's switch).
- `System resources`: Available file descriptors, sockets, free space, and inodes.
- `Core components`: Operation of `kube-state-metrics`, `node-exporter`, `kube-dns`.
- `Node state`: Cluster node state (NotReady, drain, cordon).
- `Time synchronization`: Time synchronization state on nodes.
- `CPU steal`: Cases of prolonged CPU steal exceeding.
- `Conntrack table`: Conntrack table state on nodes.
- `Pod state`: Pods with incorrect state (as a possible consequence of kubelet issues) and others.

## Ingress monitoring

Statistics collection for ingress-nginx in Prometheus is implemented with detailed metrics (response time, codes, geography, etc.), available in different dimensions (namespace, vhost, ingress). Data is visualized in Grafana with interactive dashboards.
Detailed description is in the [Ingress monitoring](../../../admin/network/alb-nginx.html#monitoring-and-statistics) section.

## Control plane monitoring

Control plane monitoring is performed using the `monitoring-kubernetes-control-plane` module, which organizes secure metrics collection and provides a basic set of monitoring rules for the following cluster components:
- `kube-apiserver`: API server monitoring and health checks.
- `kube-controller-manager`: Controller manager operation monitoring.
- `kube-scheduler`: Scheduler performance and health monitoring.
- `kube-etcd`: etcd cluster health and performance monitoring.

## Cluster monitoring

DVP securely collects monitoring metrics and configures rules.

DVP monitoring capabilities:
- `Container runtime version`: Monitoring current container runtime version (containerd) on the node and its compliance with versions allowed for use in DVP.
- `Cluster monitoring health`: Monitoring cluster monitoring subsystem health (Dead man's switch).
- `System resources`: Monitoring available file descriptors, sockets, free space, and inodes.
- `Node state`: Monitoring cluster node state (NotReady, drain, cordon).
- `Core components`: Operation of `kube-state-metrics`, `node-exporter`, `kube-dns`.
- `Time synchronization`: Monitoring time synchronization state on nodes.
- `CPU steal`: Monitoring cases of prolonged CPU steal exceeding.
- `Conntrack table`: Monitoring Conntrack table state on nodes.
- `Pod state`: Monitoring pods with incorrect state (as a possible consequence of kubelet issues).
- `Control plane components`: Control plane component monitoring (implemented by the `monitoring-kubernetes-control-plane` module).
- `Secret monitoring`: Cluster Secret monitoring and TLS certificate expiration in them (implemented by the `extended-monitoring` module).
- `Event collection`: Kubernetes cluster event collection as metrics (implemented by the `extended-monitoring` module).
- `Image availability`: Container image availability monitoring in registry used by controllers (Deployments, StatefulSets, DaemonSets, CronJobs) (implemented by the `extended-monitoring` module).
- `Extended monitoring objects`: Monitoring objects in namespaces with the `extended-monitoring.deckhouse.io/enabled=""` label (implemented by the `extended-monitoring` module).

## Extended monitoring mode

Deckhouse supports the use of extended monitoring mode, which provides alerts for additional metrics:

- `Free space and inodes`: Free space and inodes on node disks.
- `Node utilization`: Node resource utilization monitoring.
- `Pod and image availability`: Pod and container image availability monitoring.
- `Certificate expiration`: Certificate expiration monitoring.
- `Cluster events`: Other cluster events monitoring.

### Alerting in extended monitoring mode

Deckhouse provides the ability to flexibly configure alerting for each namespace and specify different severity levels depending on the threshold. You can define multiple thresholds for sending warnings to different namespaces, for example, for the following parameters:

- `Free space and inode values`: Free space and inode values on disk.
- `CPU utilization`: CPU utilization of nodes and containers.
- `5xx error percentage`: Percentage of `5xx` errors on `nginx-ingress`.
- `Unavailable pods count`: Number of potentially unavailable pods in `Deployment`, `StatefulSet`, `DaemonSet`.

## Alerts

Monitoring in Deckhouse includes event notifications. The standard delivery includes a set of basic warnings covering cluster state and its components. There is also the ability to add custom alerts.

### Sending alerts to external systems

Deckhouse supports sending alerts using `Alertmanager`:

- `SMTP protocol`: Via SMTP protocol.
- `PagerDuty`: To PagerDuty.
- `Slack`: To Slack.
- `Telegram`: To Telegram.
- `Webhook`: Via Webhook.
- `Other channels`: Via any other channels supported in Alertmanager.

## Availability assessment

Availability assessment in DVP is performed by the `upmeter` module.

Composition of the `upmeter` module:

- `agent`: Runs on master nodes and performs availability probes, sends results to the server.
- `upmeter`: Collects results and maintains an API server for their retrieval.
- `front`:
  - `status`: Shows availability level for the last 10 minutes (requires authorization, but it can be disabled).
  - `webui`: Shows a dashboard with statistics on probes and availability groups (requires authorization).
- `smoke-mini`: Maintains continuous *smoke testing* using StatefulSet.

The module sends about 100 metric readings every 5 minutes. This value depends on the number of enabled Deckhouse Virtualization Platform modules.