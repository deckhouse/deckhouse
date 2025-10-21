---
title: "Monitoring in Deckhouse Kubernetes Platform"
permalink: en/admin/configuration/monitoring/
description: "Configure comprehensive monitoring for Deckhouse Kubernetes Platform with Prometheus and Grafana. Metrics collection, alerting, dashboards, and SLA monitoring for cluster health."
---

Deckhouse Kubernetes Platform (DKP) provides a Kubernetes monitoring solution based on **Prometheus** and **Grafana**.
The module automatically configures metrics collection from nodes, pods, and key cluster components (etcd, kube-apiserver, CoreDNS), and offers preset dashboards for analyzing CPU, memory, disk, and network usage.

All components operate in a fault-tolerant mode, including Prometheus and Alertmanager, and are adapted for both cloud and bare metal environments.

The principles of operation and Prometheus configuration are covered in the [article](./prometheus.html).

Several types of monitoring are implemented:

- [Hardware resource monitoring](#hardware-resource-monitoring);
- [Kubernetes monitoring](#kubernetes-monitoring);
- [Ingress monitoring](#ingress-monitoring);
- [Network interaction monitoring](./configuring/network-and-nodes.html);
- [Extended monitoring](#extended-monitoring-mode);
- [Cluster SLA monitoring](#cluster-sla-monitoring).

[Extended monitoring mode](#extended-monitoring-mode) and [alerting](#alerts) are provided, including [sending alerts to external systems](#sending-alerts-to-external-systems). Dashboard monitoring is available, and there is cluster SLA monitoring.

## Hardware resource monitoring

Tracking of cluster hardware resource utilization is provided with graphs showing:

- CPU utilization;
- memory utilization;
- disk utilization;
- network utilization.

Graphs are available with aggregation by:

- pods;
- controllers;
- namespaces;
- nodes.

## Kubernetes monitoring

The module is designed for basic cluster node monitoring.

It provides secure metrics collection and offers a basic set of rules for monitoring:
- current container runtime version (docker, containerd) on the node and its compliance with versions allowed for use;
- overall cluster monitoring subsystem health (Dead man's switch);
- available file descriptors, sockets, free space, and inodes;
- operation of `kube-state-metrics`, `node-exporter`, `kube-dns`;
- cluster node state (NotReady, drain, cordon);
- time synchronization state on nodes;
- cases of prolonged CPU steal exceeding;
- Conntrack table state on nodes;
- pods with incorrect state (as a possible consequence of kubelet issues) and more.

## Ingress monitoring

Statistics collection for ingress-nginx in Prometheus is implemented with detailed metrics (response time, codes, geography, etc.), available in different dimensions (namespace, vhost, ingress). Data is visualized in Grafana with interactive dashboards.
Detailed description is in the [Ingress monitoring](../network/ingress/alb/nginx.html#monitoring-and-statistics) section.

## Control plane monitoring

Control plane monitoring is performed using the [`monitoring-kubernetes-control-plane`](/modules/monitoring-kubernetes-control-plane/) module, which organizes secure metrics collection and provides a basic set of monitoring rules for the following cluster components:
* kube-apiserver;
* kube-controller-manager;
* kube-scheduler;
* kube-etcd.

## Application monitoring

This monitoring is designed for automatic metrics collection from user applications in the Kubernetes cluster via Prometheus. Simply enable the [`monitoring-custom`](/modules/monitoring-custom/) module, add the `prometheus.deckhouse.io/custom-target` label to a Service or Pod and specify the port (e.g., `http-metrics`), and metrics will start being collected without manual Prometheus configuration.

The system supports flexible settings: HTTPS, custom paths, query parameters, Istio integration (mTLS), and overload protection (metrics limit).
This allows integrating applications into the general cluster monitoring, tracking their state and performance.

## Cluster monitoring

DKP securely collects monitoring metrics and configures rules.

DKP monitoring capabilities:
- monitoring current container runtime version (containerd) on the node and its compliance with versions allowed for use in DKP;
- monitoring cluster monitoring subsystem health (Dead man's switch);
- monitoring available file descriptors, sockets, free space, and inodes;
- monitoring cluster node state (NotReady, drain, cordon);
- operation of `kube-state-metrics`, `node-exporter`, `kube-dns`;
- monitoring time synchronization state on nodes;
- monitoring cases of prolonged CPU steal exceeding;
- monitoring Conntrack table state on nodes;
- monitoring pods with incorrect state (as a possible consequence of kubelet issues);
- monitoring control plane components (implemented by the `monitoring-kubernetes-control-plane` module);

## Extended monitoring mode

DKP supports the use of extended monitoring mode, which provides alerts for additional metrics:

- free space and inodes on node disks,
- node utilization,
- pod and container image availability,
- certificate expiration,
- other cluster events.

It also allows you to configure:

- monitoring secrets in the cluster (Secret objects) and TLS certificate expiration in them (implemented by the `extended-monitoring` module);
- collecting Kubernetes cluster events as metrics (implemented by the `extended-monitoring` module);
- monitoring container image availability in registry used by controllers (Deployments, StatefulSets, DaemonSets, CronJobs) (implemented by the `extended-monitoring` module);
- monitoring objects in namespaces that have the `extended-monitoring.deckhouse.io/enabled=""` label (implemented by the `extended-monitoring` module).

### Alerting in extended monitoring mode

DKP provides the ability to flexibly configure alerting for each namespace and specify different severity levels depending on thresholds. You can define multiple thresholds for sending warnings to different namespaces, for example, for the following parameters:

- free space and inode values on disk;
- CPU utilization of nodes and containers;
- percentage of `5xx` errors on `nginx-ingress`;
- number of potentially unavailable pods in `Deployment`, `StatefulSet`, `DaemonSet`.

## Alerts

Monitoring in DKP includes event notifications. The standard delivery includes a set of basic warnings covering cluster state and its components. There is also the ability to add custom alerts.

### Sending alerts to external systems

DKP supports sending alerts using `Alertmanager`:

- via SMTP protocol;
- to PagerDuty;
- to Slack;
- to Telegram;
- via Webhook;
- through any other channels supported in Alertmanager.

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
