---
title: "Overview"
permalink: en/user/monitoring/
---

This section is intended for project users of the Deckhouse Kubernetes Platform (DKP).

DKP includes a built-in monitoring system
that provides convenient tools for observing the state of the infrastructure and applications.

By default, DKP provides a predefined set of dashboards and alerts that help track key application health metrics.
They are available through the "Monitoring" section of the [Deckhouse web interface](/modules/console/).

In addition, users can:

- Collect metrics from their own applications.
- Create custom dashboards to visualize required metrics.
- Configure custom alerts and override alert thresholds.

For more information about advanced monitoring capabilities, including dashboard and metrics management,
refer to the documentation for the [`observability`](/modules/observability/) module.

## Available by default

After installing DKP, users have access to a basic set of tools for monitoring the cluster out of the box.

### Dashboards

Dashboards display charts with data on CPU usage, memory consumption, disk activity, and network traffic,
broken down by pods, nodes, or namespaces.

In the "Monitoring" → "Dashboards" section of the [Deckhouse web interface](/modules/console/),
users can access the following dashboard groups:

- **Ingress Nginx**: Metrics related to the Ingress NGINX Controller operation,
  including virtual host status, response codes, and request latency.
- **Resource consumption (Main)**: Key cluster and application metrics,
  including resource utilization and status of pods, controllers, and namespaces.
- **Security**: Metrics related to cluster security.

### Alerts

Alerts are automated notifications that report events requiring attention,
such as metric threshold violations or component availability issues.
For most alerts, trigger thresholds can be overridden if necessary.

By default, a DKP cluster includes alerts for the following types of events:

- Certificate expiration, as well as errors during certificate issuing or renewal
  (`cert-manager`, `extended-monitoring`, and `ingress-nginx` modules).
- Container image pull failures, including authentication and authorization issues, invalid image names,
  missing images in a registry, or registry unavailability (`extended-monitoring` module).
- Workload execution errors for resources such as CronJob, Deployment, DaemonSet, and StatefulSet,
  including pod creation failures, unavailable replicas, and scheduling errors (`extended-monitoring` module).
- Metric exporter unavailability, preventing Prometheus from scraping metrics (`extended-monitoring` module).
- Disk space issues, including insufficient storage space or inode exhaustion on PVCs (`extended-monitoring` module).
- Ingress NGINX Controller errors, including a high rate of `5xx` responses from backends
  (`extended-monitoring` module).
- Network performance issues (`monitoring-ping` module).

## Monitoring configuration

The following monitoring configuration options are available to DKP users:

- **Monitoring user applications**: You can configure metrics collection from your application
  by following the [instructions](app.html).
- **Creating custom dashboards**: You can add specialized dashboards using the [GrafanaDashboardDefinition](/modules/prometheus/faq.html#how-do-i-create-custom-grafana-dashboards) resource.
- **Configuring custom alerts**: You can define new alerting rules using the [CustomPrometheusRules](/modules/prometheus/faq.html#how-do-i-add-alerts-andor-recording-rules) resource.
