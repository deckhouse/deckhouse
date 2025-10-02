---
title: "Observability Module"
description: Observability
d8Edition: [ce,ee,se,se+,be]
menuTitle: "Observability"
moduleStatus: experimental
---

The `Observability` module extends the functionality of the [Prometheus module](https://deckhouse.io/products/kubernetes-platform/documentation/v1/modules/prometheus/) and the [Deckhouse Web Interface module](https://deckhouse.io/products/kubernetes-platform/modules/console/stable/). It provides more flexible tools for managing metric visualization and access control.

## Features

### Dashboard self-service management mechanism

The module introduces new types of dashboards, including namespace-level resources, allowing users to create and manage their own dashboards without requiring cluster-wide permissions.

Previously, dashboards were created only via the GrafanaDashboardDefinition resource, which required cluster-wide access rights.  
The new mechanism allows the use of resources that operate within a namespace.

The following resource types are supported:

* `ObservabilityDashboard` — namespace-scoped dashboards. Available in the Deckhouse Web Interface under "Monitoring → Projects".
* `ClusterObservabilityDashboard` — dashboards for visualizing cluster component metrics. Available in the Deckhouse Web Interface under "Monitoring → System".
* `ClusterObservabilityPropagatedDashboard` — enables extending the list of dashboards from the two categories above. These dashboards are automatically appended to the dashboard lists in the "Monitoring → System" and "Monitoring → Projects" sections of the Web Interface and become available to users with sufficient permissions for the corresponding namespaces or system section.

> Existing dashboards created with the GrafanaDashboardDefinition resource must be manually converted to the new types.

### Dashboard access control

Access to dashboards is configured using Kubernetes RBAC:

* **Read-only** access requires the `get` permission on the corresponding resource.
* **Create/edit** access requires the `create`, `update`, `patch`, and `delete` permissions.
* Permissions are granted separately for:
  * `observabilitydashboards.observability.deckhouse.io` — namespace-level dashboards.
  * `clusterobservabilitydashboards.observability.deckhouse.io` — system dashboards.
  * `clusterobservabilitypropagateddashboards.observability.deckhouse.io` — dashboards propagated to all users.

### Metrics access control

Access to metrics is also configured using RBAC. Metric filtering is automatically applied based on the user's permissions:

* Namespace users get access only to metrics within their own namespace. RBAC checks are performed against the `metrics.observability.deckhouse.io` resource.
* Platform administrators get access to all system-level metrics (`d8-*`, `kube-*`, and metrics without the `namespace` label). This is controlled via the `clustermetrics.observability.deckhouse.io` resource.  
  Metrics from user namespaces are also accessible if explicit permissions are granted via `metrics.observability.deckhouse.io`.

### Support for custom data sources

Data sources (`datasources`) previously created using the `GrafanaAdditionalDatasource` resource continue to work and are available in the Deckhouse Web Interface — in the "Data explorer" and "Dashboards" sections.

## Interfaces

Once the module is enabled, the following interfaces become available in the Deckhouse Web Interface:

* **Monitoring → System**:
  * Dashboards — view and manage system dashboards;
  * Data explorer — execute PromQL queries against all metrics;
  * Monitoring system status — display Prometheus configuration and target status.

* **Monitoring → Projects**:
  * Dashboards — view and manage dashboards within the namespace;
  * Data explorer — execute PromQL queries scoped to the namespace;
  * Monitoring system status — display status of Prometheus targets available to the user.
