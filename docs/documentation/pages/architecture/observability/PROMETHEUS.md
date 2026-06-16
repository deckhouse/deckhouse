---
title: Prometheus module
permalink: en/architecture/observability/prometheus.html
search: prometheus module, monitoring architecture, monitoring components, monitoring, metrics
description: Architecture of the prometheus modules in Deckhouse Kubernetes Platform.
---

The `prometheus` module expands the monitoring stack with preset parameters for DKP and applications, which simplifies the initial configuration.

For more details about the module, refer to [the module documentation](/modules/prometheus/) section.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`prometheus`](/modules/prometheus/) module and its interactions with other components of DKP are shown in the following diagrams:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Prometheus module architecture](../../images/architecture/observability/c4-l2-prometheus.svg)

## Module components

The module consists of the following components:

1. **Prometheus-main** (StatefulSet): Main Prometheus. [Prometheus](https://github.com/prometheus/prometheus) is a monitoring and notification system using a time series database (TSDB). It collects and analyzes application and server performance metrics in real time. Prometheus-main collects metrics from configured monitoring objects every 30 seconds. You can use the [scrapeInterval](/modules/prometheus/configuration.html#parameters-scrapeinterval) parameter to change this value.

   Prometheus-main can use the original ("vanilla") Prometheus or [Deckhouse Prom++](https://github.com/deckhouse/prompp) that is a high—performance open-source fork of Prometheus designed to significantly reduce memory consumption while maintaining full compatibility with the original project. The module uses Deckhouse Prom++ by default. It is possible to switch from Deckhouse Prom++ to the original Prometheus. In this case, migration of write-ahead log (WAL) data will be required, since Deckhouse Prom++ uses its own WAL log format. Migration is performed automatically using the prompptool init container.

   Prometheus-main is the main data source. It collects metrics, processes configured rules, and sends alerts according to its configuration. The [Prometheus Operator](/modules/operator-prometheus/) creates the Prometheus instance and its configuration based on following custom resources:
  
   * [Prometheus](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#prometheus): Describes a Prometheus installation (cluster).
   * [ServiceMonitor](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#servicemonitor): Specifies how to collect metrics from a set of services.
   * [PrometheusRule](https://github.com/coreos/prometheus-operator/blob/master/Documentation/api-reference/api.md#prometheusrule): Contains a set of Prometheus rules.

   Prometheus Operator watches Prometheus resources and, for each one, generates:

   * StatefulSet with Prometheus itself.
   * Secret containing `prometheus.yaml` (the main configuration file) and `configmaps.json` (the configuration file for `prometheus-config-reloader`, described below). The `prometheus-main` secret is mounted in prometheus-main pod and is used by the config-reloader container.

   Prometheus Operator also watches ServiceMonitor and PrometheusRule resources and, based on them, updates the configuration (`prometheus.yaml` and `configmaps.json`) in the described above Secret.

   For more details about the working of Prometheus Operator, refer to [the `operator-prometheus` module documentation](/modules/operator-prometheus/) section.

   For more details about the prometheus-main operation, refer to [the Architecture of the monitoring in DKP](monitoring.html#prometheus) section.

   Prometheus-main consists of the following containers:

   * **init-config-reloader**: Init container that performs a single run of config-reloader to load the Prometheus configuration.
   * **prompptool**: Init container that performs automatic WAL data migration in case of switching from Deckhouse Prom++ to the original Prometheus and vice versa.
   * **config-reloader**: Sidecar container that monitors changes in the `prometheus.yaml` configuration file and, if needed, triggers a Prometheus configuration reload (via a special HTTP request to the `/-/reload` endpoint). Config-reloader is a [utility](https://github.com/coreos/prometheus-operator/tree/master/cmd/prometheus-config-reloader) from the [Prometheus Operator](https://github.com/coreos/prometheus-operator/) open-source project.
   * **prometheus**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to the metrics of the proxy container. It is an [open-source project](https://github.com/brancz/kube-rbac-proxy).

2. **Prometheus-longterm** (StatefulSet): The secondary Prometheus instance that scrapes the data of the primary Prometheus instance (`prometheus-main`). This allows users to view and analyze historical trends over a long period of time. Prometheus-longterm receives data due to a configured federation with the primary Prometheus.

   The original Prometheus or Deckhouse Prom++ can be used in prometheus-longterm too. Prometheus-longterm has the same set of containers as in prometheus-main, as well as operation principles.

   {% alert level="info" %}
   Grafana-v10 will be deprecated in the near future, and you will need to use the DKP web interface to view the monitoring dashboards.
   {% endalert %}

3. **Grafana-v10**: Optional Grafana component that provides a web interface for visualizing monitoring data. Grafana displays dashboards supplied with DKP modules. Grafana could work in High Availability mode, does not store state, and is configured with [custom resources](/modules/prometheus/cr.html#grafanaadditionaldatasource). Grafana is enabled by default, but it can be disabled using [the following module parameter](/modules/prometheus/configuration.html#parameters-grafana-enabled).

   It consists of the following containers:

   * **dashboard-provisioner**: Sidecar container that watches [GrafanaDashboardDefinition](/modules/prometheus/cr.html#grafanadashboarddefinition) custom resources and when new GrafanaDashboardDefinition resource appears, it adds to the Grafana folder the dashboards described there.
   * **grafana**: Main container. It is an [open-source project](https://github.com/grafana/grafana).
   * **kube-rbac-proxy**: Sidecar container providing authorized access to controller metrics and status (described above).

4. **Aggregating-proxy**: Component that performs metrics caching, data collection from several Prometheus instances (if they are in High Availability mode), data deduplication, and query calculation.

   It consists of the following containers:

   * **wait-memcached**: Init container that waits the memcached component to be available over the network. Aggregating-proxy uses memcached to cache metrics in RAM.
   * **mimir**: Sidecar container that works with the memcached component to optimize queries and cache data. If there is no data in the cache, mimir forwards the request to the prometheus-main component via promxy, that is another sidecar container. It is an [open-source project](https://github.com/grafana/mimir).
   * **promxy**: Sidecar container that proxies requests to the prometheus-main component. Promxy is a proxy server for Prometheus which allows multiple Prometheus nodes to be a single API endpoint for the user. It is an [open-source project](https://github.com/jacksontj/promxy).
   * **kube-rbac-proxy**: Sidecar container providing authorized access to controller metrics and status (described above).

5. **Memcached** (StatefulSet): Component used by aggregating-proxy for caching Prometheus metrics. Memcached is a software that implements a service for caching data in RAM. The goal is to speed up Prometheus metrics query execution.

   It consists of the following containers:

   * **memcached**: Main container. It is an [open-source project](https://github.com/memcached/memcached).
   * **exporter**: Sidecar container that exports memcached container metrics. Exporter collects memcached container metrics via a network connection, as well as from the memcached process's PID file. It is an [open-source project](https://github.com/prometheus/memcached_exporter).

6. **Trickster**: Caching proxy server that reduces the load on Prometheus. It is used for caching and proxying prometheus-longterm requests. It will be deprecated in the near future.

   It consists of the following containers:

   * **trickster**: Main container. It is an [open-source project](https://github.com/trickstercache/trickster).
   * **kube-rbac-proxy**: Sidecar container providing authorized access to controller metrics and status (described above).

   {% alert level="info" %}
   Alerts-receiver will be removed from the [`prometheus`](/modules/prometheus/) module in the near future, Alertmanager from [`observability`](/modules/observability/) module will be used to receive all alerts.
   {% endalert %}

7. **Alerts-receiver**: A server compatible with [Alertmanager](https://github.com/prometheus/alertmanager) API. Alerts-receiver receives basic alerts from prometheus-main, creates [ClusterAlerts](/modules/prometheus/cr.html#clusteralert) custom resources based on them, updates their statuses and deletes them if the alert is no longer active. ClusterAlerts custom resources are used to inform DKP users on active alerts and are displayed in the web interface of DKP. Alerts-receiver is developed by Flant. It consists of one container.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Watches PrometheusRule GrafanaDashboardDefinition custom resources.
   * Manages ClusterAlert custom resources.
   * Authorizes requests for metrics.

2. **Alertmanager**: Sends custom alerts.

Prometheus, which is part of the module, collects metrics from all DKP components:

* Components of modules.
* Components of cluster control plane.
* Exporters that collect metrics of cluster hardware resources utilization.
* Exporters that collect Kubernetes resources metrics;
* User applications (additional configuration is required).

Prometheus interactions related to the collection of metrics from DKP components are not shown in the diagram, so as not to complicate it with a large number of relations.

The following external components interact with the module:

1. **Ingress-controller** (controller nginx as stated on diagram): Forwards users requests to Grafana.

## Fault Tolerance and High Availability Monitoring (HA) mode

The [`prometheus`](/modules/prometheus/) module provides built-in fault tolerance for all its key components. All monitoring services (Prometheus servers, storage systems, proxies, and other important components) are deployed in multiple copies by default. This ensures that in the event of a failure of a separate instance, the service will continue to work without loss of data and availability.

Prometheus, the main component of metric collection, runs in at least two copies (if there are enough nodes in the cluster). All Prometheus instances use the same configuration and receive the same data. To ensure seamless operation in case of failure of one of the copies, a special component, the aggregation proxy, is used to access Prometheus. It allows you to combine metrics from both Prometheus instances and always return the most complete and up-to-date data, even if one of the copies is temporarily unavailable.
