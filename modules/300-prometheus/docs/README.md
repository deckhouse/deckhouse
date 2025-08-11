---
title: "The Prometheus monitoring module"
type:
  - instruction
search: prometheus
description: "Monitoring of the Deckhouse Kubernetes Platform cluster using Prometheus and Grafana."
webIfaces:
- name: grafana
---

This module installs and configures the [Prometheus](https://prometheus.io/) monitoring system. Also, it configures metrics scraping for many typical applications and provides the basic set of Prometheus alerts and Grafana dashboards.

The [Vertical Pod Autoscaler](../../modules/vertical-pod-autoscaler/) module makes it possible to automatically request CPU and memory resources based on the utilization history when the Pod is recreated. Also, the Prometheus memory consumption is minimized by caching requests to it via [Trickster](https://github.com/trickstercache/trickster).

Both pulling and pushing of metrics are supported.

## Monitoring hardware resources

Prometheus tracks hardware resources and displays the corresponding utilization graphs for:

- CPU
- Memory
- Disk drive
- Network

The graphs can be grouped by:

- Pods
- Controllers
- Namespaces
- Nodes

## Kubernetes monitoring

Deckhouse configures monitoring of Kubernetes "health" parameters and its components such as:

- General cluster utilization.
- Connectivity of Kubernetes nodes to each other (RTT between all nodes is measured).
- Availability and operability of control plane components:
  - `etcd`.
  - `coredns`.
  - `kube-dns`.
  - `kube-apiserver` and others.
- Time synchronization on nodes and other parameters.

## Ingress monitoring

The detailed description is available [here](../../modules/ingress-nginx/#monitoring-and-statistics).

## Advanced monitoring mode

Deckhouse also provides [the advanced monitoring mode](../extended-monitoring/) that implements additional metrics-based alerts, such as: free space and inode-related, the node usage, the availability of Pods and container images, certificates expiration and other Kubernetes cluster events.

### Alerting in advanced monitoring mode

Deckhouse allows you to flexibly configure alerts for each namespace and specify different levels of severity based on the threshold. You can set thresholds in various namespaces for the following parameters:

- Empty space and inodes on a disk.
- CPU usage for a node and a container.
- Percent of `5xx` errors on `ingress-nginx`.
- Number of unavailable Pods in a `Deployment`, `StatefulSet`, `DaemonSet`.

## Alerts

The Deckhouse monitoring includes event notifications. The standard edition includes a set of basic alerts covering the health of the cluster and its components. Also, you can add custom alerts.

### Sending alerts to external systems

Deckhouse supports sending alerts using `Alertmanager`:

- Via the SMTP protocol
- To PagerDuty
- To Slack
- To Telegram
- Via the Webhook mechanism
- By any other means supported in Alertmanager

## Included modules

![The scheme of interaction](../../images/prometheus/prometheus_monitoring_new.svg)

### Components installed by Deckhouse

| Name                        | Description                                                                                                                                                                                                                                                                              |
|-----------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **prometheus-main**         | The primary Prometheus instance that scrapes metrics every 30 seconds (you can change this value using the `scrapeInterval` parameter). It processes all the rules, sends alerts, and serves as the main data source.                                                                    |
| **prometheus-longterm**     | The secondary Prometheus instance that scrapes the data of the primary Prometheus instance (`prometheus-main`) every 5 minutes (you can change this value using the `longtermScrapeInterval` parameter). It is used for long-term history storage and displaying data for large periods. |
| **trickster**               | The caching proxy that reduces the load on Prometheus.                                                                                                                                                                                                                                   |
| **aggregating-proxy**       | An aggregating and caching proxy  that reduces the load on Prometheus and aggregate both main and longterm in single datasource.                                                                                                                                                         |
| **memcached**               | Distributed memory caching system.                                                                                                                                                                                                                                                       |
| **grafana**                 | The managed observability platform with ready-to-use dashboards for all Deckhouse modules and popular applications. Grafana instances are highly available, stateless, and configured by CRDs.                                                                                           |
| **metrics-adapter**         | The component connecting Prometheus and Kubernetes metrics API. It enables HPA support in a Kubernetes cluster.                                                                                                                                                                          |
| **vertical-pod-autoscaler** | An autoscaling tool to help size Pods for the optimal CPU and memory resources required by the Pods.                                                                                                                                                                                     |
| **Various Exporters**       | Precooked exporters connected to Prometheus. The list includes exporters for all necessary metrics: `kube-state-metrics`, `node-exporter`, `oomkill-exporter`, `image-availability-exporter`, and many more.                                                                             |

### External components

Deckhouse has interfaces to integrate with various popular solutions in the following ways:

| Name                           | Description                                                                                                                                      |
|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| **Alertmanagers**              | Alertmanagers could be connected to Prometheus and Grafana and deployed to the Deckhouse cluster or out of it.                                   |
| **Long-term metrics storages** | Utilizing remote write protocol, it is possible to send metrics from Deckhouse to plenty of storages, including [Cortex](https://www.cortex.io/), [Thanos](https://thanos.io/), [VictoriaMetrics](https://victoriametrics.com/products/open-source/). |
