---
title: "Architecture of monitoring in the Deckhouse Kubernetes Platform"
permalink: en/architecture/monitoring/
---

## Composition and interaction scheme of monitoring components

![Interaction diagram](../../images/prometheus/prometheus_monitoring_new.svg)

### Components installed by DKP

| Component                   | Description                                                                                                                                                                                                                                                                                        |
|-----------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **prometheus-operator**     | DKP module responsible for running Prometheus in the cluster.                                                                                                                                                                                                                                   |
| **prometheus-main**         | Main Prometheus that performs scraping every 30 seconds (can be changed using the `scrapeInterval` parameter). It processes all rules, sends alerts and is the main data source.                                                                        |
| **prometheus-longterm**     | Additional Prometheus storing sparse data samples from the main prometheus-main                                                                                                                                                                                                      |
| **aggregating-proxy**       | Aggregating and caching proxy that combines main and longterm into one source. Helps avoid data gaps when one of the Prometheus instances is unavailable.                                                                                                                                     |
| **memcached**               | In-memory data caching service.                                                                                                                                                                                                                                                 |
| **grafana**                 | UI for displaying metrics in dashboard format.                                                                                                                                                                                                                                                  |
| **metrics-adapter**         | Component providing Kubernetes API for accessing metrics. Required for proper VPA operation.                                                                                                                                                                                          |
| **Various exporters**    | Set of ready-made Prometheus exporters for all necessary metrics: `kube-state-metrics`, `node-exporter`, `oomkill-exporter`, `image-availability-exporter`.                                                                                                                                     |
| **upmeter**                 | Module for assessing DKP component availability.                                                                                                                                                                                                                                                  |
| **trickster**               | Caching proxy that reduces load on Prometheus. Will be deprecated soon.                                                                                                                                                                                                       |


### External components

DKP can integrate with a large number of diverse solutions in the following ways:

| Name                       | Description|
|--------------------------------|--------------------------------------------------------------------------|
| **Alertmanagers**              | Alertmanagers can be connected to Prometheus and Grafana and be located both in the DKP cluster and outside it.|
| **Long-term metrics storages** | Using the `remote write` protocol, it is possible to send metrics from DKP to a large number of storage systems, including [Cortex](https://www.cortex.io/), [Thanos](https://thanos.io/), [VictoriaMetrics](https://victoriametrics.com/products/open-source/).|


## Prometheus

Prometheus collects metrics and executes rules:

* For each *target* (monitoring target) with a given period `scrape_interval`, Prometheus makes an HTTP request to this *target*, receives metrics in [its own format](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md) in response and saves them to its database.
* Each `evaluation_interval` processes rules (*rules*), based on which:
  * sends alerts;
  * or saves new metrics (result of rule execution) to its database.

Prometheus is installed by the `prometheus-operator` module of DKP, which performs the following functions:
- Defines the following custom resources:
  - `Prometheus`: Defines the installation (cluster) of *Prometheus*.
  - `ServiceMonitor`: Defines how to collect metrics from services.
  - `Alertmanager`: Defines the cluster of *Alertmanagers*.
  - `PrometheusRule`: Defines the list of *Prometheus rules*.
- Monitors these resources, and also:
  - Generates `StatefulSet` with *Prometheus* itself.
  - Creates secrets with configuration files necessary for Prometheus operation (`prometheus.yaml` — Prometheus configuration, and `configmaps.json` — configuration for `prometheus-config-reloader`).
  - Monitors `ServiceMonitor` and `PrometheusRule` resources and updates *Prometheus* configuration files by making changes to secrets based on them.

### What's in the Prometheus pod?

![What's in the Prometheus pod](../../images/operator-prometheus/pod.png)

* Two containers:
  * `prometheus`: Prometheus itself;
  * `prometheus-config-reloader`: [Wrapper](https://github.com/coreos/prometheus-operator/tree/master/cmd/prometheus-config-reloader) that:
    * Monitors changes to `prometheus.yaml` and, if necessary, calls configuration reload to Prometheus (via special HTTP request, see [more details below](#how-are-service-monitors-processed)).
    * Monitors PrometheusRules (see [more details below](#how-are-custom-resources-with-rules-processed)) and downloads them and restarts Prometheus as needed.
* Pod uses three volumes:
  * config: Mounted secret (two files: `prometheus.yaml` and `configmaps.json`). Connected to both containers.
  * rules: `emptyDir` that is filled by `prometheus-config-reloader` and read by `prometheus`. Connected to both containers, but in `prometheus` in read-only mode.
  * data: Prometheus data. Mounted only in `prometheus`.


### How is Prometheus configured?

* The Prometheus server has *config* and *rule files* (files with rules);
* The `config` has the following sections:
  * `scrape_configs`: Settings for finding *targets* (monitoring targets, see more details in the next section).
  * `rule_files`: List of directories where *rules* that need to be loaded are located:

    ```yaml
    rule_files:
    - /etc/prometheus/rules/rules-0/*
    - /etc/prometheus/rules/rules-1/*
    ```

  * `alerting`: Settings for finding *Alert Managers* to send alerts to. This section is very similar to `scrape_configs`, only the result of its work is a list of *endpoints* to which Prometheus will send alerts.

### Where does Prometheus get the list of *targets*?

* In general, Prometheus works as follows:

  ![Prometheus operation](../../images/operator-prometheus/targets.png)

  * **(1)** Prometheus reads the `scrape_configs` section of the config, according to which it configures its internal Service Discovery mechanism.
  * **(2)** The Service Discovery mechanism interacts with the Kubernetes API (mainly — gets endpoints).
  * **(3)** Based on what happens in Kubernetes, the Service Discovery mechanism updates Targets (list of *targets*).
* In `scrape_configs` a list of *scrape jobs* (internal Prometheus concept) is specified, each of which is defined as follows:

  ```yaml
  scrape_configs:
    # General settings
  - job_name: d8-monitoring/custom/0    # just the name of the scrape job, shown in the Service Discovery section
    scrape_interval: 30s                  # how often to collect data
    scrape_timeout: 10s                   # request timeout
    metrics_path: /metrics                # path to request
    scheme: http                          # http or https
    # Service discovery settings
    kubernetes_sd_configs:                # means that targets are obtained from Kubernetes
    - api_server: null                    # means to use the API server address from environment variables (which are in every Pod)
      role: endpoints                     # take targets from endpoints
      namespaces:
        names:                            # search for endpoints only in these namespaces
        - foo
        - baz
    # "Filtering" settings (which endpoints to take and which not) and "relabeling" (which labels to add or remove, on all received metrics)
    relabel_configs:
    # Filter by the value of the prometheus_custom_target label (obtained from the service associated with the endpoint)
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: .+                           # any NON-empty label matches
      action: keep
    # Filter by port name
    - source_labels: [__meta_kubernetes_endpointslice_port_name]
      regex: http-metrics                 # matches only if the port is called http-metrics
      action: keep
    # Add job label, use the value of the prometheus_custom_target label of the service, to which we add the prefix "custom-"
    #
    # The job label is a Prometheus service label:
    #    * it determines the name of the group in which the target will be shown on the targets page
    #    * and of course it will be on each metric obtained from these targets, so you can easily filter in rules and dashboards
    - source_labels: [__meta_kubernetes_service_label_prometheus_custom_target]
      regex: (.*)
      target_label: job
      replacement: custom-$1
      action: replace
    # Add namespace label
    - source_labels: [__meta_kubernetes_namespace]
      regex: (.*)
      target_label: namespace
      replacement: $1
      action: replace
    # Add service label
    - source_labels: [__meta_kubernetes_service_name]
      regex: (.*)
      target_label: service
      replacement: $1
      action: replace
    # Add instance label (which will contain the Pod name)
    - source_labels: [__meta_kubernetes_pod_name]
      regex: (.*)
      target_label: instance
      replacement: $1
      action: replace
  ```

* Thus, Prometheus itself tracks:
  * Addition and removal of Pods (when Pods are added/removed, Kubernetes changes endpoints, and Prometheus sees this and adds/removes *targets*).
  * Addition and removal of services (more precisely endpoints) in specified namespaces.
* Config changes are required in the following cases:
  * Need to add a new scrape config (usually — a new type of services that need to be monitored).
  * Need to change the list of namespaces.

### How are Service Monitors processed?

![How Service Monitors are processed](../../images/operator-prometheus/servicemonitors.png)

1. Prometheus Operator reads (and also monitors addition/removal/changes) Service Monitors (which specific Service Monitors — specified in the `prometheus` resource itself, see more details in [official documentation](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api-reference/api.md)).
1. For each Service Monitor, if it does NOT specify a specific list of namespaces (specified `any: true`), Prometheus Operator calculates (by accessing the Kubernetes API) a list of namespaces where there are Services (matching the labels specified in the Service Monitor).
1. Based on the read `servicemonitor` resources (see [official documentation](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api-reference/api.md#servicemonitorspec)) and based on the calculated namespaces, Prometheus Operator generates part of the config (the `scrape_configs` section) and saves the config to the corresponding Secret.
1. Using standard Kubernetes means, data from the secret comes to the Pod (the `prometheus.yaml` file is updated).
1. The `prometheus-config-reloader` notices the file change and sends an HTTP request to Prometheus to reload.
1. Prometheus re-reads the config and sees changes in scrape_configs, which it processes according to its own logic (see more details above).

### How are custom resources with *rules* processed?

![How custom resources with rules are processed](../../images/operator-prometheus/rules.png)

1. Prometheus Operator monitors PrometheusRules (matching the `ruleSelector` specified in the `prometheus` resource).
1. If a new PrometheusRule appears (or an existing one is deleted) — Prometheus Operator updates `prometheus.yaml` (and then the logic exactly corresponding to Service Monitor processing, which is described above, is triggered).
1. Both in case of adding/removing PrometheusRule and when changing the content of PrometheusRule, Prometheus Operator updates the ConfigMap `prometheus-main-rulefiles-0`.
1. Using standard Kubernetes means, data from ConfigMap comes to the Pod
1. The `prometheus-config-reloader` notices the file change and:
   - Downloads changed ConfigMaps to the rules directory (this is `emptyDir`).
   - Sends an HTTP request to Prometheus to reload.
1. Prometheus re-reads the config and sees the changed *rules*.

## Architecture of DKP component availability assessment (upmeter)

Availability assessment in DKP is performed by the `upmeter` module.

Composition of the `upmeter` module:

- **agent**: Works on master nodes and performs availability probes, sends results to the server.
- **upmeter**: Collects results and maintains an API server for their extraction.
- **front**:
  - **status**: Shows availability level for the last 10 minutes (requires authorization, but it can be disabled);
  - **webui**: Shows a dashboard with statistics on probes and availability groups (requires authorization).
- **smoke-mini**: Maintains continuous *smoke testing* using StatefulSet.

The module sends about 100 metric readings every 5 minutes. This value depends on the number of enabled Deckhouse Kubernetes Platform modules.
