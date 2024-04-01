---
title: "The prometheus-metrics-adapter module: usage"
search: autoscaler, HorizontalPodAutoscaler
---

{% raw %}

Note that only HPA (Horizontal Pod Autoscaling) with [apiVersion: autoscaling/v2](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.23/#objectmetricsource-v2-autoscaling), whose support has been available since Kubernetes v1.12, is discussed below.

Configuring HPA requires:
* defining what is being scaled (`.spec.scaleTargetRef`);
* defining the scaling range (`.spec.minReplicas`, `.scale.maxReplicas`);
* defining metrics to be used as the basis for scaling (`.spec.metrics`) and registering them with the Kubernetes API.

Metrics in terms of HPA are of three types:
* [classic](#classic-scaling-by-custom-resource-consumption) — of type (`.spec.metrics[].type`) "Resource"; these are used for simple scaling based on CPU and memory consumption;
* [custom](#scaling-by-custom-metrics) — of type (`.spec.metrics[].type`) "Pods" or "Object";
* [external](#apply-external-metrics-to-hpa) — of type (`.spec.metrics[].type`) "External".

**Caution!** [By default,](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#default-behavior) HPA uses different approaches for scaling:
* If the metrics [indicate](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) that scaling **up** is required, it is done immediately (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 0). The only limitation is the rate of increase: pods can double in 15 seconds, but if there are less than 4 pods, 4 new pods will be added.
* If the metrics [indicate](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#algorithm-details) that scaling **down** is required, it happens within 5 minutes (`spec.behavior.scaleUp.stabilizationWindowSeconds` = 300): suggestions for a new number of replicas are calculated, then the largest value is selected. There is no limit on the number of pods to be removed at once.

If metrics are subject to fluctuations that result in a surge of unnecessary application replicas, the following approaches are used:
* Wrapping the metric with an aggregation function (e. g., `avg_over_time()`) if the metric is defined by a PromQL query. For more details, see. [example](#example-use-unstable-custom-metrics).
* Increasing the stabilization window (parameter `spec.behavior.scaleUp.stabilizationWindowSeconds`) in the _HorizontalPodAutoscaler_ resource. During the this period, requests to increase the number of replicas will be accumulated, then the most modest request will be selected. This method is identical to applying the `min_over_time(<stabilizationWindowSeconds>)` aggregation function, but only if the metric is increasing and scaling **up** is required. For scaling **down**, the default settings usually work good enough. For more details, see [example](#classical-scaling-by-resource-consumption).
* Limiting the rate of increase of the new replica count with `spec.behavior.scaleUp.policies`.

## Scaling types

The following metrics can be used to scale applications:
1. [Classic metrics](#classic-resource-consumption-based-scaling).
1. [Custom namespace-scoped metrics](#scaling-based-on-custom-metrics). This type is suitable if you have a single application, the source of the metrics is in the namespace and it is tied to one of the objects.
1. [Custom cluster-wide metrics](#scaling-based-on-custom-metrics). This type is suitable if you have many applications using the same metric, whose source is in the application namespace, and it is associated with one of the objects. Such metrics let you put common infrastructure components into a separate deployment ("infra").
1. If the metric source is not tied to the application namespace, you can use [external](#using-external-metrics-in-hpa) metrics. For example, metrics provided by a cloud provider or an external SaaS service.

**Caution!** We recommend using option 1 ([classic](#classic-scaling-by-consumption-resources) metrics), or option 2 ([custom](#scaling-by-custom-metrics) metrics defined in the _Namespace_). In this case, we suggest defining the application configuration (including its autoscaling) in the app repository. You should consider options 3 and 4 only if you have a large collection of identical microservices.

## Classic resource consumption-based scaling

Below is an example HPA configuration for scaling based on the classic metrics from `metrics.k8s.io`: CPU and memory utilization for pods. The `averageUtulization` value reflects the target percentage of resources that have been **requested**.

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  # Indicates the controller to be scaled (reference to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  # Controller scaling limits.
  minReplicas: 1
  maxReplicas: 10
  # If the application is prone to short-term spikes in CPU consumption,
  # you can postpone the scaling decision to see if it is necessary.
  # By default, scaling up occurs immediately.
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
  metrics:
  # CPU- and memory-based scaling.
  - type: Resource
    resource:
      name: cpu
      target:
        # Scaling will occur when the average CPU utilization of all pods in scaleTargetRef exceeds the specified value.
        # For a metric with type: Resource, only type: Utilization is available.
        type: Utilization
        # Scaling will occur if 1 core is requested for all Deployment pods and if over 700m is already in use on average.
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        # Example of a scaling rule when the average memory usage of all pods in scaleTargetRef exceeds the given value.
        type: Utilization
        # Scaling will occur if 1 GB of memory is requested for the pods and more than 800 MB is already in use on average.
        averageUtilization: 80
```

## Scaling based on custom metrics

### Registering custom metrics with Kubernetes API

Custom metrics must be registered with the `/apis/custom.metrics.k8s.io/` API, this registration is done by prometheus-metrics-adapter (and it also implements the API). The metrics can then be referenced using the _HorizontalPodAutoscaler_ object. Customizing a vanilla prometheus-metrics-adapter is a time-consuming process. We made it easier by defining a set of [Custom Resources](cr.html) with different Scopes:
* Namespaced:
  * `ServiceMetric`;
  * `IngressMetric`;
  * `PodMetric`;
  * `DeploymentMetric`;
  * `StatefulsetMetric`;
  * `NamespaceMetric`;
  * `DaemonSetMetric` (unavailable to users).
* Cluster:
  * `ClusterServiceMetric` (unavailable to users);
  * `ClusterIngressMetric` (unavailable to users);
  * `ClusterPodMetric` (unavailable to users);
  * `ClusterDeploymentMetric` (unavailable to users);
  * `ClusterStatefulsetMetric` (unavailable to users);
  * `ClusterDaemonSetMetric` (unavailable to users).

You can use the cluster-wide resource to define the metric globally, and use the _Namespace_ to redefine it locally. [Format](cr.html) is the same for all custom resources.

### Using custom metrics in HPA

Once a custom metric is registered, it can be referenced. In terms of HPA, custom metrics can be of two types — `Pods` and `Object`.

`Object` refers to an object in the cluster that has metrics in Prometheus with corresponding labels (`namespace=XXX,ingress=YYYYY`). These labels will be substituted in place of `<<.LabelMatchers>>` in your custom request.

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
  # Specifies the controller to be scaled (reference to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  # Metrics to use for scaling.
  # An example of using custom metrics.
  metrics:
  - type: Object
    object:
      # An object that has metrics in Prometheus.
      describedObject:
        apiVersion: networking.k8s.io/v1
        kind: Ingress
        name: myingress
      metric:
        # A metric registered with the IngressMetric or ClusterIngressMetric custom resource.
        # You can use rps_1m, rps_5m, or rps_15m that come with the prometheus-metrics-adapter module.
        name: mymetric
      target:
        # You can use `Value` or `AverageValue` for metrics of type Object.
        type: AverageValue
        # Scaling occurs if the average value of the custom metric for all pods in the Deployment deviates significantly from 10.
        averageValue: 10
```

`Pods` — all pods will be selected from the resource managed by HPA and metrics will be collected for each pod with the relevant labels (`namespace=XXX`, `pod=YYYY-sadiq`, `namespace=XXX`, `pod=YYYY-e3adf`, etc.). Then the HPA will calculate an average value based on these metrics and will use it for [scaling](#examples-involving-custom-metrics-of-type-pods).

#### Using custom metrics with the RabbitMQ queue size

In the example below, scaling is performed based on the `send_forum_message` queue in RabbitMQ for which the `rmq` service is registered. If the number of messages in this queue exceeds 42, scaling is carried out.

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
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The controller to be scaled (reference to a deployment or statefulset).
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

#### Using volatile custom metrics

This example improves on the previous one.

In the example below, scaling is based on the `send_forum_message` queue in RabbitMQ, for which the `rmq` service is registered. If the number of messages in this queue exceeds 42, scaling takes place. The MQL function `avg_over_time()` smoothes (averages the metric) to avoid over-scaling due to short-term spikes in the number of messages.

```yaml
apiVersion: deckhouse.io/v1beta1
kind: ServiceMetric
metadata:
  name: rmq-queue-forum-messages
  namespace: mynamespace
spec:
  query: sum (avg_over_time(rabbitmq_queue_messages{<<.LabelMatchers>>,queue=~"send_forum_message",vhost="/"}[5m])) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The controller to be scaled (reference to a deployment or statefulset).
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

#### Examples involving custom metrics of type `Pods`

In the example below, the number of workers is scaled based on the percentage of active php-fpm workers.
The trigger is the average number of php-fpm-workers in `mybackend` _Deployment_, which should not exceed 5.

```yaml
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-workers
spec:
  query: sum (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) by (<<.GroupBy>>)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The controller to be scaled (reference to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: mybackend
  minReplicas: 1
  maxReplicas: 5
  metrics:
  # HPA has to loop through all the Deployment pods and collect metrics from them.
  - type: Pods
    # Unlike type: Object, you don't have to specify describedObject.
    pods:
      metric:
        # A custom metric registered using the PodMetric custom resource.
        name: php-fpm-active-workers
      target:
        # For metrics of type: Pods, only AverageValue can be used.
        type: AverageValue
        # The scaling will take place if the average metric value of all Deployment pods exceeds 5.
        averageValue: 5
```

Scaling the Deployment based on the percentage number of active php-fpm-workers:

```yaml
---
apiVersion: deckhouse.io/v1beta1
kind: PodMetric
metadata:
  name: php-fpm-active-worker
spec:
  # Percentage of active php-fpm-workers. The round() function gets rid of millipercentages in HPA.
  query: round(sum by(<<.GroupBy>>) (phpfpm_processes_total{state="active",<<.LabelMatchers>>}) / sum by(<<.GroupBy>>) (phpfpm_processes_total{<<.LabelMatchers>>}) * 100)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: {{ .Chart.Name }}-hpa
spec:
  # The cpntroller to be scaled (reference to a deployment or statefulset).
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
        # Scaling will take place if, on average, the Deployment has 80% of the workers in use.
        averageValue: 80
```

### Registering external metrics with the Kubernetes API

The `prometheus-metrics-adapter` module supports the `externalRules` mechanism. It allows you to define custom PromQL queries and register them as metrics.

A universal rule that allows you to create your own metrics without customization in `prometheus-metrics-adapter` has been added in the installation examples — "any metric in Prometheus with the name `kube_adapter_metric_<name>` will be registered in the API under the name `<name>`". Then, you just need to write an exporter that will export such a metric or create a [recording rule](https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/) in Prometheus that will aggregate your metric based on other metrics.

Below is an example of _CustomPrometheusRules_:

The example showcases Prometheus custom rules for the `mymetric` metric.

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # The recommended template for naming your CustomPrometheusRules.
  name: prometheus-metrics-adapter-mymetric
spec:
  groups:
  # The recommended template
  - name: prometheus-metrics-adapter.mymetric
    rules:
    # The name of your new metric
    # Note that the 'kube_adapter_metric_' prefix is mandatory.
    - record: kube_adapter_metric_mymetric
      # A request with results that will end up in the final metric; there's no point in attaching extra labels to it.
      expr: sum(ingress_nginx_detail_sent_bytes_sum) by (namespace,ingress)
```

### Using external metrics in HPA

Once an external metric is registered, you can refer to it.

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The controller to be scaled (reference to a deployment or statefulset).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:
  # Scaling based on the external metrics
  - type: External
    external:
      metric:
        # The metric we registered by creating a metric in Prometheus (kube_adapter_metric_mymetric), but without the prefix 'kube_adapter_metric_'.
        name: mymetric
        selector:
          # For external metrics, you can and should refine the request with labels.
          matchLabels:
            namespace: mynamespace
            ingress: myingress
      target:
        # Only `type: Value` can be used for metrics of type External.
        type: Value
        # Scaling will take place if the value of our metric exceeds 10.
        value: 10
```

### Using the queue size in Amazon SQS

To install an exporter to integrate with SQS:
1. Create a dedicated "service" Git repository ( alternatively, you could use, e.g., an "infrastructure" repository).
1. Copy the exporter installation and the script to it — these will be used to create the necessary _CustomPrometheusRules_.

That's it, you have integrated the cluster. In case you need to configure autoscaling for just one application (in a single namespace), we recommend installing the exporter together with that application and using `NamespaceMetrics`.

The following is an example of an exporter (e. g., [sqs-exporter](https://github.com/ashiddo11/sqs-exporter)) to retrieve metrics from Amazon SQS if:
* a `send_forum_message` queue is running in Amazon SQS;
* scaling is done when the number of messages in that queue exceeds 42.

```yaml
apiVersion: deckhouse.io/v1
kind: CustomPrometheusRules
metadata:
  # The recommended name — prometheus-metrics-adapter-<metric name>.
  name: prometheus-metrics-adapter-sqs-messages-visible
  # Pay attention!
  namespace: d8-monitoring
  labels:
    # Pay attention!
    prometheus: main
    # Pay attention!
    component: rules
spec:
  groups:
  - name: prometheus-metrics-adapter.sqs_messages_visible # the recommended template
    rules:
    - record: kube_adapter_metric_sqs_messages_visible # Pay attention! The 'kube_adapter_metric_' prefix is required.
      expr: sum (sqs_messages_visible) by (queue)
---
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # The targets of scaling (link to a deployment or statefulset).
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
        # Must match CustomPrometheusRules record name without 'kube_adapter_metric_' prefix.
        name: sqs_messages_visible
        selector:
          matchLabels:
            queue: send_forum_messages
      target:
        type: Value
        value: 42
```

## Debugging

### How do I get a list of custom metrics?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/
```

### How do I get the value of a metric associated with an object?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/services/*/my-service-metric
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/ingresses/*/rps_1m
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/ingresses/*/mymetric
```

### How do I get the value of a metric created via `NamespaceMetric`?

```shell
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/my-namespace/metrics/my-ns-metric
```

### How do I get external metrics?

```shell
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1
kubectl get --raw /apis/external.metrics.k8s.io/v1beta1/namespaces/d8-ingress-nginx/d8_ingress_nginx_ds_cpu_utilization
```

{% endraw %}
