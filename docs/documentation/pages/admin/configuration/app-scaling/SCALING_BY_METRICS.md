---
title: "Scaling by metrics"
permalink: en/admin/configuration/app-scaling/scaling-by-metrics.html
description: "Configure metric-based scaling in Deckhouse Kubernetes Platform. Custom metrics scaling, Prometheus integration, and HPA configuration for dynamic resource adjustment."
---

## Scaling based on metrics

Metric-based scaling is the process of automatically or manually adjusting resources (e.g., number of pod replicas, allocated CPU/memory) based on defined metrics. These metrics can range from CPU or memory usage to the number of requests per second or the size of a message queue.

Scaling can be based on both standard and custom monitoring metrics. For example, scaling can be triggered by the following metrics:

- Pod CPU — current CPU usage.
- Pod Memory — current memory usage.
- RPS (Ingress) — number of requests per second over 1, 5, or 15 minutes (`rps_1m`, `rps_5m`, `rps_15m`).
- Average CPU usage (Pod) — over 1, 5, or 15 minutes (`cpu_1m`, `cpu_5m`, `cpu_15m`).
- Average memory usage (Pod) — over 1, 5, or 15 minutes (`memory_1m`, `memory_5m`, `memory_15m`).

### Configuration

To simplify metric-based scaling configuration, DKP offers special resources (`Cluster*Metric` and `*Metric`). You can also configure scaling [based on custom metric calculation rules](#configuring-metrics-via-customprometheusrules).

To configure scaling based on metrics, follow these steps:

1. Enable the [`prometheus-metrics-adapter`](/modules/prometheus-metrics-adapter/) module. It allows you to define custom resources that describe metric calculation rules using PromQL queries.

   You can enable the `prometheus-metrics-adapter` module in the Deckhouse web interface or via the following command:

   ```shell
   d8 platform module enable prometheus-metrics-adapter
   ```

1. Define a PromQL query that describes the desired metric (e.g., application request rate or CPU usage over time). This query is registered in the cluster as a metric for HPA.

1. Create a HorizontalPodAutoscaler) resource, specifying the metric name and threshold value. Kubernetes will query the result of your PromQL expression and compare it to the threshold, adjusting the number of replicas as needed (see the [HPA configuration documentation](hpa.html#hpa-configuration) for more details).

Example PromQL query:

```yaml
query: sum(rate(ingress_nginx_detail_requests_total{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>) OR on() vector(0)
```

In this example:

- `ingress_nginx_detail_requests_total` — the base metric (a request counter, e.g., from the Ingress NGINX controller);
- `rate(...[2m])` — calculates the rate of increase over the past 2 minutes (requests per second);
- `sum(...) by (<<.GroupBy>>)` — aggregates the results by a set of labels (e.g., Ingress name, namespace, etc.);
- `OR on() vector(0)` — a PromQL construct that ensures the result is `0` when there’s no data (otherwise, the query might return nothing, which would be ignored).

This PromQL query effectively provides the current requests per second value filtered by specified labels (Ingress, namespace, etc.).

Once an administrator defines a metric using a PromQL query (e.g., in an IngressMetric object or a similar mechanism), it can be referenced in a HorizontalPodAutoscaler (HPA) as shown below:

```yaml
apiVersion: deckhouse.io/v1beta1
kind: IngressMetric
metadata:
  name: mymetric
  namespace: mynamespace
spec:
  query: sum(rate(ingress_nginx_detail_requests_total{<<.LabelMatchers>>}[2m])) by (<<.GroupBy>>) OR on() vector(0)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The controller to scale (reference to a Deployment or StatefulSet).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  # Metrics used for scaling.
  metrics:
  - type: Object
    object:
      # The object that owns the metrics in Prometheus.
      describedObject:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        name: myingress
      metric:
        # The metric registered via a custom resource like IngressMetric or ClusterIngressMetric.
        # You can also use built-in metrics like rps_1m, rps_5m, or rps_15m provided by the prometheus-metrics-adapter module.
        name: mymetric
      target:
        # For Object-type metrics, use either `Value` or `AverageValue`.
        type: AverageValue
        # Scaling will occur if the average value of the metric across all pods in the Deployment differs significantly from 10.
        averageValue: 10
```

> When the metric exceeds (or falls below) the threshold `averageValue: 10`, the system will adjust the number of replicas of `myapp` within the range of 1 to 2.

### Configuring metrics via CustomPrometheusRules

If you need to define custom metric calculation rules without using the built-in DKP `Cluster*Metric` and `*Metric` resources, you can use the [CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules) resource.

Example:

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # Recommended naming pattern for your CustomPrometheusRules.
  name: prometheus-metrics-adapter-mymetric
spec:
  groups:
  # Recommended group name pattern.
  - name: prometheus-metrics-adapter.mymetric
    rules:
    # Name of your custom metric.
    # Important! The prefix 'kube_adapter_metric_' is required.
    - record: kube_adapter_metric_mymetric
      # PromQL query that defines the metric. Avoid including unnecessary labels.
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress)
```

> All metrics with the `kube_adapter_metric_` prefix are automatically registered in the Kubernetes API without the need to create CustomPrometheusRules. This allows you to use existing Prometheus metrics for scaling without additional configuration.

### Working with unstable metrics

When dealing with unstable metrics (e.g., metrics that fluctuate and cause excessive scaling), it is recommended to:

- Use PromQL aggregation functions. For example, `avg_over_time()` smooths the metric over a specified time range, helping to avoid sudden spikes:

  ```yaml
  apiVersion: deckhouse.io/v1beta1
  kind: ServiceMetric
  metadata:
    name: rmq-queue-forum-messages
    namespace: mynamespace
  spec:
    query: sum (avg_over_time(rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}[5m])) by (<<.GroupBy>>)
  ```

- Configure stabilization behavior in the autoscaler. You can increase the stabilization window so that decisions to scale up or down are based on more stable data.

### Retrieving metric values

To get a list of available metrics, use the following command:

```console
d8 k get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

To retrieve metric values associated with specific objects, use:

```console
d8 k get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
```

To get values of metrics created via `NamespaceMetric`, use:

```console
d8 k get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

To retrieve external metrics, use:

```console
d8 k get --raw /apis/external.metrics.k8s.io/v1beta1
```

### Configuring autoscaling in the Deckhouse Web Interface

In Deckhouse Kubernetes Platform, you can configure node autoscaling settings through the [Deckhouse web interface](/modules/console/). This allows you to dynamically change the number of nodes depending on the load.

To configure autoscaling in the web interface:

- Open the Deckhouse web interface;
- In the left menu, go to the “Nodes” → “Node Groups” section;
- Select the desired NodeGroup;
- In the “Autoscaling Parameters” section, the current settings will be displayed.

The following settings are available in the Deckhouse web interface:

- Nodes per zone – sets the minimum and maximum number of nodes that can run in a single zone.
- Desired – the current number of nodes needed for operation.
- Requested – the number of nodes currently scheduled for provisioning.
- Standby – additional reserved nodes.

To modify autoscaling parameters:

- Click the edit icon next to the “Nodes per zone” parameter.
- In the pop-up window, specify the minimum and maximum number of nodes.
- Click “Apply” to save the changes, or “Cancel” to discard them.

When you click “Create based on” in the web interface, a form for configuring autoscaling parameters opens:

1. **Main parameters:**
   - Zones – selection of availability zones where machines will be created. This helps distribute nodes across multiple zones. The default value depends on the selected cloud provider and usually includes all zones in the region.
   - Scaling priority – used if multiple node groups exist with the same priority. If not specified, the system will randomly choose a group for scaling.
   - Number of machines created simultaneously during scale-up – defines how many machines can be added at once during scale-up. For example, if the value is 1, nodes will be added one at a time.
   - Max unavailable instances during RollingUpdate – how many nodes can be unavailable during updates. A value of 0 means rolling updates will occur one by one with no downtime.

1. **Standby nodes and resources:**
   - Number of standby nodes out of total nodes – you can specify an exact number (e.g., 2–6) or a percentage (e.g., 15%). A standby node is a pre-provisioned node ready to receive workloads immediately. This speeds up scale-up by several minutes.
   - Resources reserved on standby nodes – the percentage of resources to reserve on standby nodes. Acceptable values: 1% to 80%. For example, if set to 50%, half of the node’s resources will be reserved.

1. **Additional parameters:**
   - Quick Shutdown – enables fast node shutdown, reducing the drain wait time to 5 minutes.
