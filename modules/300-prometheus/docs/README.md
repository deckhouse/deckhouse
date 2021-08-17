---
title: "The Prometheus monitoring module"
type:
  - instruction
search: prometheus
---

This module installs and configures the [Prometheus](https://prometheus.io/) monitoring system. Also, it configures metrics scraping for many typical applications and provides the basic set of Prometheus alerts and Grafana dashboards.

The module installs two Prometheus instances:
* **main** — the primary Prometheus instance that scrapes metrics every 30 seconds (you can change this value using the `scrapeInterval` parameter). It processes all the rules, sends alerts, and serves as the main data source.
* **longterm** — the secondary Prometheus instance that scrapes the data of the main instance every 5 minutes (you can change this value using the `longtermScrapeInterval` parameter). It is used for long-term history storage and for displaying data for large periods.

If a storage class supports automatic volume expansion (allowVolumeExpansion: true), it can automatically expand the volume if there is not enough disk space for Prometheus data. Otherwise, you will receive an alert that the volume space in Prometheus is running out.

The [Vertical Pod Autoscaler](../../modules/302-vertical-pod-autoscaler/) module makes it possible to automatically request cpu and memory resources based on the utilization history when the Pod is recreated. Also, the Prometheus memory consumption is minimized by caching requests to it via trickster.

Both pulling and pushing of metrics are supported.

## Monitoring hardware resources
Prometheus tracks hardware resources and displays the corresponding utilization graphs for:
- CPU,
- memory,
- disk drive,
- network.

The graphs can be grouped by:
- Pods,
- controllers,
- namespaces,
- nodes.

## Kubernetes monitoring

Deckhouse enables monitoring of a large set of "health" parameters of Kubernetes and its components out-of-the-box, including:
- overall cluster utilization;;
- connectivity of Kubernetes nodes (by measuring RTT between nodes);
- availability and operability of control plane components:
  - `etcd`
  - `coredns` and `kube-dns`
  - `kube-apiserver`, etc.
- time synchronization on nodes, etc.

## Ingress monitoring

The detailed description is available [here](../../modules/402-ingress-nginx/#monitoring-and-statistics).

## Advanced monitoring mode
Deckhouse also provides the advanced monitoring mode that implements custom metrics-based alerts. The following exporters are supported:
- `extended-monitoring-exporter`. Implements extended scraping of metrics for namespaces (that have the `extended-monitoring.flant.com/enabled=””` annotation attached), including information about available inodes/space on disks, monitoring the node usage, the availability of Deployment, `StatefulSet`, `DaemonSet` Pods, etc.;
- `image-availability-exporter`.  Adds metrics (and send alerts) for tracking the availability of the container image specified in the `image` field in the Pod's spec in `Deployments`, `StatefulSets`, `DaemonSets`, `CronJobs`.

### Alerting in advanced monitoring mode
Deckhouse allows you to flexibly configure alerting for each namespace and assign criticality depending on the threshold value. You can set thresholds in various namespaces for parameters such as:
- empty space and inodes on a disk;
- CPU usage for a node and a container;
- number of 5xx errors on `nginx-ingress`;
- number of unavailable Pods in a `Deployment`, `StatefulSet`, `DaemonSet`.

## Alerts
The Deckhouse monitoring also implements event notifications. The standard edition includes a large set of necessary alerts for monitoring the state of the cluster and its components. At the same time, you can add custom alerts.

### Sending alerts to external systems
Deckhouse supports sending alerts using `Alertmanager`:
- via the SMTP protocol;
- to PagerDuty;
- to Slack;
- to Telegram;
- via the Webhook mechanism;
- and by any other means supported in alertmanager.
