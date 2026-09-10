---
title: "Configuring a system for collecting and storing metrics"
permalink: en/admin/configuration/monitoring/prometheus.html
description: "Configure Prometheus metrics collection and storage in Deckhouse Kubernetes Platform. Deckhouse Prom++ setup, metrics configuration, and monitoring system management."
---

{{< alert level="info" >}}
Starting from version 1.71, Deckhouse Kubernetes Platform uses [Deckhouse Prom++](/products/prompp/) instead of Prometheus.
{{< /alert >}}

## Prometheus capabilities

Prometheus collects metrics and executes rules:

* For each *target* (monitoring target) at a specified interval `scrape_interval`, Prometheus makes an HTTP request to this *target*, receives metrics in its [own format](https://github.com/prometheus/docs/blob/main/docs/instrumenting/exposition_formats.md) in response, and stores them in its database.
* Every `evaluation_interval` it processes rules (*rules*), based on which:
  * it sends alerts;
  * or stores new metrics (result of rule execution) in its database.

## Prometheus operation

Prometheus is installed by the [`prometheus`](/modules/prometheus/) module of DKP, which performs the following functions:
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

## Using the `global.modules.storageClass` parameter for Prometheus

This module uses the global [`global.modules.storageClass`](../../../reference/api/global.html#parameters-modules-storageclass) parameter as the default StorageClass when creating a new PersistentVolumeClaim (if the `storageClass` parameter is not specified in the module's configuration).

If a Prometheus PVC already exists, changing the global parameter does not affect the existing PVC or trigger disk recreation. The module continues to use the existing PVC and its StorageClass.

If the module configuration defines its own `storageClass` or `longtermStorageClass` value, the module-specific value takes precedence over the global default.

{{< alert level="warning" >}}
Changing the `storageClass` or `longtermStorageClass` parameter in the module configuration deletes and recreates the existing PVC. All data will be lost. Back up your data before making this change.
{{< /alert >}}

A complete description of all settings is available in the [prometheus module documentation](/modules/prometheus/configuration.html).
