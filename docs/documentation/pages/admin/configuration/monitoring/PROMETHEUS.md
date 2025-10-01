---
title: "Configuring a system for collecting and storing metrics"
permalink: en/admin/configuration/monitoring/prometheus.html
description: "Configure Prometheus metrics collection and storage in Deckhouse Kubernetes Platform. Deckhouse Prom++ setup, metrics configuration, and monitoring system management."
lang: en
---

{% alert %}
Starting from version 1.71, Deckhouse Kubernetes Platform uses [Deckhouse Prom++](/products/prompp/) instead of Prometheus.
{% endalert %}

## What does Prometheus do?

Prometheus collects metrics and executes rules:

* For each *target* (monitoring target) at a specified interval `scrape_interval`, Prometheus makes an HTTP request to this *target*, receives metrics in its [own format](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md) in response, and stores them in its database.
* Every `evaluation_interval` it processes rules (*rules*), based on which:
  * it sends alerts;
  * or stores new metrics (result of rule execution) in its database.

## How does Prometheus work?

Prometheus is installed by the `prometheus-operator` module of DKP, which performs the following functions:
- Defines the following custom resources:
  - `Prometheus`: Defines the *Prometheus* installation (cluster).
  - `ServiceMonitor`: Defines how to collect metrics from services.
  - `Alertmanager`: Defines the *Alertmanager* cluster.
  - `PrometheusRule`: Defines the list of *Prometheus rules*.
- Monitors these resources, and also:
  - Generates `StatefulSet` with *Prometheus* itself.
  - Creates secrets with configuration files necessary for Prometheus operation (`prometheus.yaml` — Prometheus configuration, and `configmaps.json` — configuration for `prometheus-config-reloader`).
  - Monitors `ServiceMonitor` and `PrometheusRule` resources and updates *Prometheus* configuration files by modifying secrets based on them.

The module can be enabled using the following ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: prometheus
spec:
  version: 2
  enabled: true
  settings:
    auth:
      password: xxxxxx
    retentionDays: 7
    storageClass: rbd
    nodeSelector:
      node-role/monitoring: ""
    tolerations:
    - key: dedicated.deckhouse.io
      operator: Equal
      value: monitoring
```

A complete description of all settings is available in the [prometheus module documentation](/modules/prometheus/configuration.html).

## How is Prometheus configured?

* The Prometheus server has a *config* and *rule files* (files with rules)
* The `config` contains the following sections:
  * `scrape_configs`: Settings for finding *targets* (monitoring targets, see the next section for details).
  * `rule_files`: List of directories containing *rules* that need to be loaded:

    ```yaml
    rule_files:
    - /etc/prometheus/rules/rules-0/*
    - /etc/prometheus/rules/rules-1/*
    ```

  * `alerting`: Settings for finding *Alert Managers* to send alerts to. This section is very similar to `scrape_configs`, only the result of its work is a list of *endpoints* to which Prometheus will send alerts.
