---
title: "The prometheus-metrics-adapter module: Custom resources"
search: autoscaler, HorizontalPodAutoscaler 
---

{% capture cr_spec %}
* `.metadata.name` — the name of the metric (used in HPA).
* `.spec.query` — a custom PromQL query that returns a unique value for your label set (you can use `sum() by()`, `max() by()`, etc., operators for grouping). The following keys **must be used** in the request:
  * `<<.LabelMatchers>>` — will be replaced with a set of `{namespace="mynamespace",ingress="myingress"}` labels. You can add your own comma-separated labels list (as in the example [below](usage.html#example-of-using-rabbitmq-queue-size-based-custom-metrics)).
  * `<<.GroupBy>>` — will be replaced with `namespace,ingress` labels for grouping (`max() by(...)`, `sum() by (...)`, etc.).
{% endcapture %}

Setting up a vanilla `prometheus-metrics-adapter` is a time-consuming process. Happily, we have somewhat simplified it by defining a set of **CRDs** with different Scopes.

You can globally define a metric using the Cluster resource, while the Namespaced resource allows you to redefine it locally. All CRs have the same format.

## Namespaced Custom resources

### `ServiceMetric`

{{ cr_spec }}

### `IngressMetric`

{{ cr_spec }}

### `PodMetric`

{{ cr_spec }}

### `DeploymentMetric`

{{ cr_spec }}

### `StatefulSetMetric`

{{ cr_spec }}

### `NamespaceMetric`

{{ cr_spec }}

### `DaemonSetMetric` (not available to users)

{{ cr_spec }}

## Cluster Custom resources

### `ClusterServiceMetric` (not available to users)

{{ cr_spec }}

### `ClusterIngressMetric` (not available to users)

{{ cr_spec }}

### `ClusterPodMetric` (not available to users)

{{ cr_spec }}

### `ClusterDeploymentMetric` (not available to users)

{{ cr_spec }}

#### Example

### `ClusterStatefulSetMetric` (not available to users)

{{ cr_spec }}

#### Example

### `ClusterDaemonSetMetric` (not available to users)

{{ cr_spec }}
