---
title: "The prometheus-metrics-adapter module: Custom resources"
search: autoscaler, HorizontalPodAutoscaler 
---

{% capture cr_spec %}
* `.metadata.name` — the name of the metric (used in HPA).
* `.spec.query` — a custom PromQL query that returns a unique value for your label set (you can use `sum() by()`, `max() by()`, etc., operators for grouping). The following keys **must be used** in the request:
  * `<<.LabelMatchers>>` — will be replaced with a set of `{namespace="mynamespace"###PLACEHOLDER###}` labels. You can add your own comma-separated labels list (as in the [example](usage.html#using-custom-metrics-with-the-rabbitmq-queue-size)).
  * `<<.GroupBy>>` — will be replaced with `namespace###PLACEHOLDER2###` labels for grouping (`max() by(...)`, `sum() by (...)`, etc.).
{% endcapture %}

Setting up a vanilla `prometheus-metrics-adapter` is a time-consuming process. Happily, we have somewhat simplified it by defining a set of **CustomResourceDefinitions** with different scopes.

You can globally define a metric using the cluster-wide resource, while the namespaced resource allows you to redefine it locally. All custom resources have the same format.

## Namespaced custom resources

### `ServiceMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',service="myservice"'  | replace: '###PLACEHOLDER2###', ',service' }}

### `IngressMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',ingress="myingress"' | replace: '###PLACEHOLDER2###', ',ingress' }}

### `PodMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',pod="mypod-xxxxx"' | replace: '###PLACEHOLDER2###', ',pod' }}

### `DeploymentMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',deployment="mydeployment"' | replace: '###PLACEHOLDER2###', ',deployment' }}

### `StatefulSetMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ',statefulset="mystatefulset"' | replace: '###PLACEHOLDER2###', ',statefulset' }}

### `NamespaceMetric`

{{ cr_spec | replace: '###PLACEHOLDER###', ''  | replace: '###PLACEHOLDER2###', '' }}

### `DaemonSetMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',daemonset="mydaemonset"' | replace: '###PLACEHOLDER2###', ',daemonset' }}

## Cluster custom resources

### `ClusterServiceMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',service="myservice"'  | replace: '###PLACEHOLDER2###', ',service' }}

### `ClusterIngressMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',ingress="myingress"' | replace: '###PLACEHOLDER2###', ',ingress' }}

### `ClusterPodMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',pod="mypod-xxxxx"' | replace: '###PLACEHOLDER2###', ',pod' }}

### `ClusterDeploymentMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',deployment="mydeployment"' | replace: '###PLACEHOLDER2###', ',deployment' }}

### `ClusterStatefulSetMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',statefulset="mystatefulset"' | replace: '###PLACEHOLDER2###', ',statefulset' }}

### `ClusterDaemonSetMetric` (not available to users)

{{ cr_spec | replace: '###PLACEHOLDER###', ',daemonset="mydaemonset"' | replace: '###PLACEHOLDER2###', ',daemonset' }}
