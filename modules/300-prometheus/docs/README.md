---
title: "The Prometheus monitoring module"
type:
  - instruction
search: prometheus
description: "Monitoring of the Deckhouse Kubernetes Platform cluster using Prometheus and Grafana."
webIfaces:
- name: grafana
---

The module expands the monitoring stack with preset parameters for DKP and applications, which simplifies the initial configuration.

Module features:

- The package includes ready—made triggers and dashboards, and supports push and pull models for collecting metrics. The load is optimized by using caches and [Dekhouse Prom++](/products/prompp/).
- The load is optimized by using caches and Deckhouse Prom++.
- It is possible to store historical data using downsampling.
- The module covers all the basic tasks of basic monitoring of the platform and applications.

The module covers all the basic tasks of basic monitoring of DKP and applications.

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

## Monitoring Management as code (IaC approach)

The capabilities of the DKP monitoring system can be expanded by using the [Observability module](/modules/observability/stable/). It is used to implement:

- managing alerts;
- differentiation of access rights to settings and monitoring data;
- centralized dashboard management.

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

## Architecture

![The scheme of interaction](images/prometheus_monitoring.svg)

### Basic monitoring components

| Name                                      | Description                                                                                                                                                                                                                                                                                                                                           |
|-------------------------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **prometheus-main**                       | The primary Prometheus instance that scrapes metrics every 30 seconds (you can change this value using the `scrapeInterval` parameter). It processes all the rules, sends alerts, and serves as the main data source.                                                                                                                                 |
| **prometheus-longterm**                   | The secondary Prometheus instance that scrapes the data of the primary Prometheus instance (`prometheus-main`) every 5 minutes (you can change this value using the `longtermScrapeInterval` parameter). It is used for long-term history storage and displaying data for large periods.                                                              |
| **trickster**                             | The caching proxy that reduces the load on Prometheus.                                                                                                                                                                                                                                                                                                |
| **aggregating-proxy**                     | An aggregating and caching proxy  that reduces the load on Prometheus and aggregate both main and longterm in single datasource.                                                                                                                                                                                                                      |
| **memcached**                             | Distributed memory caching system.                                                                                                                                                                                                                                                                                                                    |
| **grafana**                               | The managed observability platform with ready-to-use dashboards for all Deckhouse modules and popular applications. Grafana instances are highly available, stateless, and configured by CRDs.                                                                                                                                                        |
| **metrics-adapter**                       | The component connecting Prometheus and Kubernetes metrics API. It enables HPA support in a Kubernetes cluster.                                                                                                                                                                                                                                       |
| **vertical-pod-autoscaler**               | An autoscaling tool to help size Pods for the optimal CPU and memory resources required by the Pods.                                                                                                                                                                                                                                                  |
| **Various Exporters**                     | Precooked exporters connected to Prometheus. The list includes exporters for all necessary metrics: `kube-state-metrics`, `node-exporter`, `oomkill-exporter`, `image-availability-exporter`, and many more.                                                                                                                                          |
| **Push/pull the metric collection model** | Monitoring uses a pull model by default. Data is collected from applications at the initiative of the monitoring system. The push model is also supported: metrics can be transmitted via the Prometheus Remote Write protocol or using the [Prometheus Pushgateway](/modules/prometheus-pushgateway/). |

### External components

Deckhouse has interfaces to integrate with various popular solutions in the following ways:

| Name                           | Description                                                                                                                                      |
|--------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------|
| **Alertmanagers**              | Alertmanagers could be connected to Prometheus and Grafana and deployed to the Deckhouse cluster or out of it.                                   |
| **Long-term metrics storages** | Utilizing remote write protocol, it is possible to send metrics from Deckhouse to plenty of storages, including [Cortex](https://www.cortex.io/), [Thanos](https://thanos.io/), [VictoriaMetrics](https://victoriametrics.com/products/open-source/). |

## Fault Tolerance and High Availability Monitoring (HA) mode

The monitoring module provides built-in fault tolerance for all key DKP components. All monitoring services (Prometheus servers, storage systems, proxies, and other important components) are deployed in multiple copies by default. This ensures that in the event of a failure of a separate instance, the service will continue to work without loss of data and availability.

Prometheus, the main component of metric collection, runs in at least two copies (if there are enough nodes in the cluster). Both Prometheus instances use the same configuration and receive the same data. To ensure seamless operation in case of failure of one of the copies, a special component, the aggregation proxy, is used to access Prometheus. It allows you to combine metrics from both Prometheus instances and always return the most complete and up-to-date data, even if one of the copies is temporarily unavailable.
