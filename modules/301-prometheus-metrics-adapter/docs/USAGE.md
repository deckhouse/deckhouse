---
title: "The prometheus-metrics-adapter module: usage"
search: autoscaler, HorizontalPodAutoscaler 
---

Below, only HPAs of the [apiVersion: autoscaling/v2beta2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.20/#horizontalpodautoscalerspec-v2beta2-autoscaling) type (supported from Kubernetes v1.12 onward) are considered.

To configure an HPA, you need to:
* determine the scaling target (`.spec.scaleTargetRef`),
* define the scaling range (`.spec.minReplicas`, `.scale.maxReplicas`),
* define the metrics that will be used for scaling and register them with the Kubernetes API (`.spec.metrics`).

There are three types of metrics in terms of an HPA:
* [classic](#classic-resource-consumption-based-scaling) — these have the "Resource" type (`.spec.metrics[].type`) and are used to scale based on memory and CPU consumption;
* [custom](#custom-metrics-based-scaling) — these have the "Pods" or "Object" type (`.spec.metrics[].type`).
* [external](#external-metrics-based-scaling) — these have the "Resource" type (`.spec.metrics[].type`).

## What scaling type should I prefer?

1. The typical use-cases of a [classic](#classic-resource-consumption-based-scaling) type are pretty obvious.
1. Suppose you have a single application, the source of metrics is located inside the Namespace, and it is associated with one of the objects. In this case, we recommend using the [custom](#custom-metrics-based-scaling) Namespace-scoped metrics.
1. Use [custom](#custom-metrics-based-scaling) Cluster-wide metrics if multiple applications use the same metric associated with one of the objects, and the metric's source belongs to the Application Namespace. Such metrics can help you combine common infrastructure components into a separate ("infra") deployment.
1. Use [external](#external-metrics-based-scaling) metrics if the source of the metric does not belong to the App Namespace. These can be, for example, cloud provider or SaaS-related metrics.

**Caution!** We strongly recommend using either 1. [classic](#classic-resource-consumption-based-scaling) metrics or 2. [custom](#custom-metrics-based-scaling) metrics defined in the Namespace. In this case, you can define the entire configuration of the application (including the autoscaling logic) in the repository of the application. Options 3 and 4 should only be considered if you have a large collection of identical microservices.

## Classic resource consumption-based scaling

Below is an example of the HPA for scaling based on standard `metrics.k8s.io`metrics (CPU and memory of the Pods). Please, take special note of the `averageUtulization` — this value reflects the target percentage of resources that have been **requested**.

{% raw %}
```yaml
apiVersion: autoscaling/v2beta2
kind: HorizontalPodAutoscaler
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  # what is being scaled?
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  # "min and max values
  minReplicas: 1
  maxReplicas: 10
  metrics:
  # scaling based on CPU and Memory consumption
  - type: Resource
    resource:
      name: cpu
      target:
        # scale up if the average CPU utilization by all the Pods in scaleTargetRef exceeds the specified value
        type: Utilization      # for type: Resource metrics only the type: Utilization parameter is available
        averageUtilization: 70 # scale up if all the Deployment's Pods have requested 1 CPU core and consumed more than 700m on average
  - type: Resource
    resource:
      name: memory
      target:
        # scale up if the average Memory utilization by all the Pods in scaleTargetRef exceeds the specified value
        type: Utilization
        averageUtilization: 80 # scale up if all the Deployment's Pods have requested 1 GB and consumed more than 800MB on average
```
{% endraw %}

## Custom metrics-based scaling

### Registering custom metrics with the Kubernetes API

Custom metrics must be registered with the `/apis/custom.metrics.k8s.io/` API. In our case, `prometheus-metrics-adapter` (it also implements the API) performs the registration. The `HorizontalPodAutoscaler` object can refer to these metrics after the registration is complete. Setting up a vanilla  `prometheus-metrics-adapter` is a time-consuming process. Happily, we have somewhat simplified it by defining a set of [Custom Resources](cr.html) with different Scopes:
* Namespaced:
    * `ServiceMetric`
    * `IngressMetric`
    * `PodMetric`
    * `DeploymentMetric`
    * `StatefulsetMetric`
    * `NamespaceMetric`
    * `DaemonSetMetric` (not available to users)
* Cluster:
    * `ClusterServiceMetric` (not available to users)
    * `ClusterIngressMetric` (not available to users)
    * `ClusterPodMetric` (not available to users)
    * `ClusterDeploymentMetric` (not available to users)
    * `ClusterStatefulsetMetric` (not available to users)
    * `ClusterDaemonSetMetric` (not available to users)

You can globally define a metric using the Cluster-scoped resource, while the Namespaced resource allows you to redefine it locally. All CRs have the same [format](cr.html).


### Using custom metrics with HPA

After a custom metric is registered, you can refer to it. For the HPA, custom metrics can be of two types — `Pods` and `Object`. `Object` is a reference to a cluster object that has metrics with the appropriate labels (`namespace=XXX,ingress=YYY`) in Prometheus. These labels will be substituted instead of `<<.LabelMatchers>>` in your custom request.

{% raw %}
```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # the targets of scaling (link to a deployment or statefulset).
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:              # What metrics to use for scaling. We use custom metrics of the Object type.
  - type: Object
    object:
      describedObject:  # Some object that has metrics in Prometheus.
        apiVersion: extensions/v1beta1
        kind: Ingress
        name: myingress
      metric:
        name: mymetric  # The metric registered using IngressMetric or ClusterIngressMetric CRs.
      target:
        type: Value     # Only `type: Value` can be used for metrics of the Object type.
        value: 10       # Scale up if the value of our custom metric is greater than 10
```
{% endraw %}

In the case of the `Pods` metric type, the process is more complex. First, metrics with the appropriate labels (`namespace=XXX,pod=YYY-sadiq`,`namespace=XXX,pod=YYY-e3adf`,...) will be collected for all the Pods of the resource to scale. Next, HPA will calculate the average value based on these metrics and will use it for scaling. See the example [below](#examples-of-using-custom-metrics-of-the-pods-type).

## Example of using RabbitMQ queue size-based custom metrics

Suppose there is a "send_forum_message" queue in RabbitMQ, and this message broker is exposed as an "rmq" service. Then, suppose, we want to scale up the cluster if there are more than 42 messages in the queue.

{% raw %}
```yaml
apiVersion: deckhouse.io/v1beta1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myconsumer
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Object
    object:
      describedObject:
        apiVersion: v1
        kind: Service
        name: rmq
      metric:
        name: rmq-queue-forum-messages
      target:
        type: Value
        value: 42
```
{% endraw %}

## Examples of using custom metrics of the `Pods` type

Suppose we want the average number of php-fpm workers in the "mybackend" deployment to be no more than 5.

{% raw %}
```yaml
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # Choosing what to scale
    apiVersion: apps/v1
    kind: Deployment
    name: mybackend
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: Pods # HPA must go through all the Pods in the Deployment and collect metrics from them.
    pods:      # You do not need to specify descripedObject (in contrast to type: Object).
      metric:
        name: php-fpm-active-workers # We registered this custom metric using the PodMetric CR.
      target:
        type: AverageValue # For type: Pods metrics, the AverageValue can only be used.
        averageValue: 5   # Scale up if the average metric value for all the Pods of the myworker Deployment is greater than 5.
```
{% endraw %}

The deployment is scaled based on the percentage of active php-fpm workers.

{% raw %}
```yaml
---
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-worker
spec:
  query: round(sum by(<<.GroupBy>>) (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) / sum by(<<.GroupBy>>) (phpfpm_processes_total{<<.LabelMatchers>>}) * 100) # Percentage of active php-fpm workers. The round() function rounds the percentage.
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: {{ .Chart.Name }}-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1beta1
    kind: Deployment
    name: {{ .Chart.Name }}
  minReplicas: 4
  maxReplicas: 8
  metrics:
  - type: Pods
    pods:
      metric:
        name: php-fpm-active-worker
      target:
        type: AverageValue
        averageValue: 80 # Scale up if, on average, 80% of workers in the deployment are running at full capacity 
```
{% raw %}

### Registering external metrics with the Kubernetes API

Prometheus-metrics-adapter supports the `externalRules` mechanism. Using it, you can create custom PromQL requests and register them as metrics. In our installations, we have implemented a universal rule that allows you to create your metrics without using prometheus-metrics-adapter — "any Prometheus metric called `kube_adapter_metric_<name>` will be registered in the API under the `<name>`". In other words, all you need is to either write an exporter (to export the metric) or create a [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) in Prometheus that will aggregate your metric based on other metrics.

An example of CustomPrometheusRules:

{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: prometheus-metrics-adapter-mymetric # The recommended template for naming your CustomPrometheusRules.
spec:
  groups:
  - name: prometheus-metrics-adapter.mymetric # recommended template
    rules:
    - record: kube_adapter_metric_mymetric # the name of the new metric. Pay attention! The 'kube_adapter_metric_' prefix is required.
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress) # The results of this request will be passed to the final metric; there is no reason to include excess labels into it.
```
{% endraw %}

### Using external metrics with HPA

You can refer to a metric after it is registered.

{% raw %}
```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:       # the targets of scaling (link to a deployment or statefulset).
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:              # What metrics to use for scaling. We use external metrics.
  - type: External
    external:
      metric:
        name: mymetric  # The metric that we registered by creating a metric in Prometheus's kube_adapter_metric_mymetric but without 'kube_adapter_metric_' prefix
        selector:
          matchLabels:  # For external metrics, you can and should specify matching labels.
            namespace: mynamespace
            ingress: myingress
      target:
        type: Value     # Only `type: Value` can be used for metrics of the External type.
        value: 10       # Scale up if the value of our metric is greater than 10.
```
{% endraw %}

### Example of scaling based on the Amazon SQS queue size

**Caution!** Note that an exporter is required to integrate with SQS. For this, create a separate "service" git repository (or you can use an "infrastructure" repository) and put the installation of this exporter as well as the script to create the necessary CustomPrometheusRules into this repository. If you need to configure autoscaling for a single application (especially if it runs in a single namespace), we recommend putting the exporter together with the application and using NamespaceMetrics.

Suppose there is a "send_forum_message" queue in Amazon SQS. Then, suppose, we want to scale up the cluster if there are more than 42 messages in the queue. Also, you will need an exporter to collect Amazon SQS metrics (say, [sqs-exporter](https://github.com/ashiddo11/sqs-exporter)).

{% raw %}
```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  name: prometheus-metrics-adapter-sqs-messages-visible # the recommended name — prometheus-metrics-adapter-<metric name>
  namespace: d8-monitoring # Pay attention!
  labels:
    prometheus: main # Pay attention!
    component: rules # Pay attention!
spec:
  groups:
  - name: prometheus-metrics-adapter.sqs_messages_visible # the recommended template
    rules:
    - record: kube_adapter_metric_sqs_messages_visible # Pay attention! The 'kube_adapter_metric_' prefix is required.
      expr: sum (sqs_messages_visible) by (queue)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2beta2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myconsumer
  minReplicas: 1
  maxReplicas: 5
  metrics:
  - type: External
    external:
      metric:
        name: sqs_messages_visible # Must match CustomPrometheusRules record name without 'kube_adapter_metric_' prefix.
        selector:
          matchLabels:
            queue: send_forum_messages
      target:
        type: Value
        value: 42
```
{% endraw %}

## Debugging

### How do I get a list of custom metrics?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

### How do I get the value of a metric associated with an object?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
```

### How do I get the value of a metric created via `NamespaceMetric`?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

### How do I get external metrics?

```shell
kubectl get --raw "/apis/external.metrics.k8s.io/v1beta1"
```
