---
title: "Monitoring dashboards"
permalink: en/user/monitoring/dashboards.html
---

In this section, you will learn how to work with dashboards
to analyze the state of the Deckhouse Kubernetes Platform (DKP) and the applications running in it.

Dashboards are collections of charts and tables that display application performance data.
They provide information about CPU usage, memory consumption, disk and network activity,
as well as the status of Pods, controllers, nodes, and namespaces.

## Dashboard types

DKP features preinstalled and custom dashboards, which can be created in several ways.

| Dashboard type | Description |
| ------ | -------- |
| [Preinstalled](#preinstalled-dashboards) | Ready-to-use dashboards that are already installed in DKP. Designed to monitor the state of running applications. |
| [Custom dashboards created using the `observability` module](#using-the-observability-module) | Custom dashboards created using the ObservabilityDashboard resource at the namespace level, with support for access control.<br><br>This is the recommended way to work with dashboards. |
| [Custom dashboards created using GrafanaDashboardDefinition](#using-grafanadashboarddefinition) | Custom dashboards created using the GrafanaDashboardDefinition resource at the cluster level. They require elevated privileges and do not support access control.<br><br>This is a legacy approach that will be deprecated in future DKP versions. |

## Preinstalled dashboards

DKP users have access to a basic set of dashboards for monitoring the state of running applications.
Dashboards are available in the [Deckhouse web UI](/modules/console/) under "Monitoring" → "Dashboards".

{% alert level="info" %}
Preinstalled dashboards are not available for editing.
{% endalert %}

### Ingress Nginx

Dashboards for monitoring the operation of the Ingress controller.
They include metrics reflecting the state of virtual hosts, HTTP response statistics, and request processing latency data.

Available dashboards:

- **Namespaces**: Aggregated Ingress resource metrics by namespace.
- **Namespace Detail**: Detailed information about Ingress resources in a selected namespace.
- **VHosts**: An overview of the state of virtual hosts.
- **VHost Detail**: Detailed information about a selected virtual host.

### Resource consumption (Main)

A set of dashboards for analyzing application resource consumption.
They are intended for load assessment, resource issue detection, and workload health analysis.

Available dashboards:

- **Namespaces**: Summary information for all namespaces.
- **Namespace**: Key resource usage metrics for a selected namespace.
- **Namespace / Controller**: Resource usage statistics for controllers within a selected namespace.
- **Namespace / Controller / Pod**: Detailed metrics for individual Pods.

### Security

Dashboards that contain metrics related to the security of cluster components.

Available dashboards:

- **Admission policy engine** — metrics of the [`admission-policy-engine`](/modules/admission-policy-engine/) module,
  including information about policy checks and enforcement.

## Custom dashboards

DKP users can create custom dashboards in several ways,
depending on the requirements for access control and the dashboard scope.

### Using the observability module

The [`observability`](/modules/observability/) module extends the functionality of the `prometheus` module
and the Deckhouse web UI by providing additional capabilities for flexible metric visualization and access control.

The module introduces new dashboard types, including namespace-scoped resources.
This allows users to create and manage their own dashboards without requiring permissions for cluster-level objects.
The module also simplifies dashboard editing by allowing users to configure dashboards directly in the web UI,
without manually managing resources.

{% alert level="info" %}
Before using these resources, make sure that the `observability` module is enabled in the cluster.
If necessary, contact your DKP administrator.
{% endalert %}

The following resources are available for creating dashboards:

- [ObservabilityDashboard](/modules/observability/cr.html#observabilitydashboard): Dashboards scoped to a namespace.
  They are displayed in the Deckhouse web UI under "Monitoring" → "Projects".

  Example:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ObservabilityDashboard
  metadata:
    name: example-dashboard
    namespace: my-namespace
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example dashboard"
  spec:
    definition: |
      {
        "title": "Example dashboard",
        ...
      }
  ```

- [ClusterObservabilityDashboard](/modules/observability/cr.html#clusterobservabilitydashboard): Dashboards for visualizing cluster components.
  They are displayed in the Deckhouse web UI under "Monitoring" → "System".

  Example:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ClusterObservabilityDashboard
  metadata:
    name: example-dashboard
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example dashboard"
  spec:
    definition: |
      {
        "title": "Example dashboard",
        ...
      }
  ```

- [ClusterObservabilityPropagatedDashboard](/modules/observability/cr.html#clusterobservabilitypropagateddashboard): Dashboards that extend the list of dashboards from the two categories above.
  Such dashboards are automatically added to the Deckhouse web UI
  and are displayed under "Monitoring" → "System and Monitoring" → "Projects".
  They become available to users who have permissions for the corresponding namespace or system section.

  Example:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ClusterObservabilityPropagatedDashboard
  metadata:
    name: example-dashboard
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example dashboard"
  spec:
    definition: |
      {
        "title": "Example dashboard",
        ...
      }
  ```

#### Access control

Access to dashboards is configured using the mechanisms of the [current role-based access control (RBAC) model](../../admin/configuration/access/authorization/rbac-current.html).

Depending on the dashboard type (system or user), permissions are granted for the following resources:

- `observabilitydashboards.observability.deckhouse.io`: Namespace-scoped dashboards.
- `clusterobservabilitydashboards.observability.deckhouse.io`: System dashboards.
- `clusterobservabilitypropagateddashboards.observability.deckhouse.io`: Dashboards propagated to all users.

The following permissions are required to perform operations on dashboards:

- reading: `get`
- creating and editing: `create`, `update`, `patch`, `delete`

Access to metrics used in dashboards is also controlled via RBAC.
Depending on the granted permissions, metric filtering is performed automatically.

The following access scenarios are supported:

- Namespace users can access only the metrics of their own namespace.
  RBAC access to the `metrics.observability.deckhouse.io` resource is checked.

- DKP administrators have access to all system metrics:
  - Deckhouse metrics (`d8-*`)
  - Kubernetes metrics (`kube-*`)
  - metrics without the `namespace` label

  RBAC access to the `clustermetrics.observability.deckhouse.io` resource is used.

- Metrics from user namespaces can also be available to administrators
  if they have the appropriate permissions for the `metrics.observability.deckhouse.io` resource.

Example configuration of ClusterRole and RoleBinding resources for read and edit access to metrics and dashboards:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: observability-editor
rules:
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["metrics", "observabilitydashboards"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["observability.deckhouse.io"]
    resources: ["observabilitydashboards"]
    verbs: ["create", "update", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: bind-observability-editor
  namespace: my-namespace
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: observability-editor
subjects:
  - kind: User
    name: user@example.com
    apiGroup: rbac.authorization.k8s.io
```

#### Converting dashboards from GrafanaDashboardDefinition

To migrate dashboards created using the legacy GrafanaDashboardDefinition resource
to one of the formats supported by the `observability` module, edit each dashboard manifest manually.
Pay attention to the following key differences:

| GrafanaDashboardDefinition format | `observability` module format |
| ------------------ | -------- |
| The Grafana folder is specified in the `spec.folder` field. | The folder is specified using the `observability.deckhouse.io/category` annotation. |
| The dashboard title is specified in the `title` field of the JSON manifest. | The title is specified using the `observability.deckhouse.io/title` annotation. If the annotation is missing, the `title` field from the JSON manifest is used. |

Conversion example:

- Old format:

  ```yaml
  apiVersion: deckhouse.io/v1
  kind: GrafanaDashboardDefinition
  metadata:
    name: example-dashboard
  spec:
    folder: "Apps"
    json: '{
      "title": "Example Dashboard",
      ...
    }'
  ```

- New format:

  ```yaml
  apiVersion: observability.deckhouse.io/v1alpha1
  kind: ObservabilityDashboard
  metadata:
    name: example-dashboard
    namespace: my-namespace
    annotations:
      metadata.deckhouse.io/category: "Apps"
      metadata.deckhouse.io/title: "Example Dashboard"
  spec:
    definition: |
      {
        "title": "Example Dashboard",
        ...
      }
  ```

### Using GrafanaDashboardDefinition

{% alert level="info" %}
This is a legacy approach and is not recommended for new dashboards.
Support for this method will be removed in future DKP versions.
{% endalert %}

To add a dashboard directly to Grafana, use the [GrafanaDashboardDefinition](/modules/prometheus/cr.html#grafanadashboarddefinition) resource.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: GrafanaDashboardDefinition
metadata:
  name: my-dashboard
spec:
  folder: My folder # The folder in Grafana where the dashboard will be displayed.
  definition: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "limit": 100,
...
```

When using this method, consider the following limitations:

- Dashboards added via GrafanaDashboardDefinition cannot be modified through the Grafana UI.

- Alerts configured in the Dashboard panel do not work with datasource templates — such dashboards are considered invalid and are not imported.
  Starting with Grafana 9.0, legacy alerting has been deprecated and replaced with Grafana Alerting.
  Therefore, using legacy alerting (dashboard panel alerts) in dashboards is not recommended.

- If the dashboard does not appear in Grafana after applying the resource, the dashboard JSON file may contain an error.
  To view the logs of the component responsible for dashboard provisioning, use the following command:

  ```shell
  d8 k logs -n d8-monitoring deployments/grafana-v10 dashboard-provisioner
  ```
