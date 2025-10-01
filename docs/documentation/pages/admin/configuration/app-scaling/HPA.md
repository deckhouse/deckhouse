---
title: "Horizontal pod autoscaling"
permalink: en/admin/configuration/app-scaling/hpa.html
description: "Configure Horizontal Pod Autoscaler (HPA) in Deckhouse Kubernetes Platform. Automatic pod scaling based on CPU, memory, and custom metrics for optimal resource utilization."
---

## How Horizontal Scaling (HPA) works

Horizontal Pod Autoscaler (HPA) is a mechanism for automatically adjusting (up or down) the number of pod replicas (in Deployments or StatefulSets) based on metrics retrieved via the Kubernetes API. HPA monitors application load by checking current metrics (e.g., CPU, memory, or custom Prometheus metrics), and adjusts the number of replicas as needed to maintain a desired level of performance or to optimize resource usage.

## Available metric types for HPA

Horizontal scaling in DKP can be based on any available metrics, such as:

1. [Pod CPU and memory usage](hpa.html#scaling-based-on-cpu-and-memory).
   - Configured using the [HorizontalPodAutoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/) resource.  
     For example, you can define a Resource-type metric with `averageUtilization = 70` for CPU, so that the application scales up when average CPU usage reaches 70%.

1. [DKP object metric](hpa.html#scaling-based-on-object-metrics) (Ingress, Service) or pod-based metrics (sum or average across all pods of a controller).
   - Enables scaling based on metrics attached to DKP objects (e.g., Ingress, Service), or metrics aggregated from pods (sum or average per controller). Resources like ServiceMetric and IngressMetric are used for this.

1. [Any other metrics, including external data](hpa.html#scaling-based-on-external-data) (e.g., Amazon SQS metrics, cloud load balancers, SaaS services, etc.).
   - Useful when the metric source is outside the cluster (e.g., Amazon SQS, cloud Load Balancer, SaaS services). Configured using the [CustomPrometheusRules](/modules/prometheus/cr.html#customprometheusrules) resource.

## HPA recommendations

If metrics fluctuate, wrap the metric in an aggregation function (e.g., `avg_over_time()`) or increase the stabilization window (`spec.behavior.scaleUp.stabilizationWindowSeconds`) to avoid rapid scaling of pods.

## HPA limitations

1. By default, HPA handles scaling up and scaling down differently:

   - Scaling up:
     - Happens immediately (`spec.behavior.scaleUp.stabilizationWindowSeconds = 0`).
     - Limit — within 15 seconds, the number of pods can double. If there were fewer than 4 pods, up to 4 new pods can be added.

   - Scaling down:
     - Takes up to 5 minutes (`spec.behavior.scaleDown.stabilizationWindowSeconds = 300`).
     - Multiple "proposals" for a new number of replicas are collected, and the highest one is selected to avoid frequent downsizing.
     - There are no limits on how many pods can be removed at once.

1. Only one HPA per controller. You cannot assign multiple HPAs to the same Deployment (or StatefulSet) — they will conflict.

## How to enable or disable HPA

HPA does not require separate activation in DKP. However, if you want to scale based on metrics other than CPU and memory, you must enable the [`prometheus-metrics-adapter`](/modules/prometheus-metrics-adapter/) module. See how to enable it [in the documentation](scaling-by-metrics.html#how-to-enable-prometheus-metrics-adapter).

## HPA configuration

To configure HPA, follow these steps:

1. Define the controller (Deployment or StatefulSet) to scale.

1. Set scaling limits (`minReplicas` and `maxReplicas`):

   ```yaml
   minReplicas: 1
   maxReplicas: 10
   ```

1. Configure metrics:

   ```yaml
   metrics:
   - type: Resource
     resource:
       name: cpu
       target:
         type: Utilization
         averageUtilization: 70
    ```

1. Optionally, set the `stabilizationWindowSeconds` to delay scaling decisions and limit the pod growth rate:

   ```yaml
   behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
    # This delays scaling up decisions and limits the speed of pod growth.
   ```

## HPA configuration examples

### Scaling based on CPU and memory

Scaling occurs when the average CPU or memory usage across all pods exceeds the specified percentage:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: app-hpa
  namespace: app-prod
spec:
  # Specifies the controller to scale (reference to a Deployment or StatefulSet).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: app
  # Scaling boundaries for the controller.
  minReplicas: 1
  maxReplicas: 10
  # If the application tends to have short CPU usage spikes,
  # you can delay the scaling decision to confirm it's necessary.
  # By default, scaling up happens immediately.
  behavior:
    scaleUp:
      stabilizationWindowSeconds: 300
  metrics:
  # CPU and memory-based scaling.
  - type: Resource
    resource:
      name: cpu
      target:
        # Scaling happens when the average CPU utilization across all pods in scaleTargetRef exceeds this value.
        # For metrics of type: Resource, only target type: Utilization is available.
        type: Utilization
        # Example: if each pod requests 1 core, scaling occurs when average usage exceeds 700m.
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        # Scaling based on memory usage exceeding a certain percentage.
        type: Utilization
        # Example: if each pod requests 1 GiB, scaling occurs when average usage exceeds 800 MiB.
        averageUtilization: 80
```

### Scaling based on object metrics

If the `rmq-queue-forum-messages` metric (number of messages in RabbitMQ) exceeds 42, the HPA increases the number of replicas in the `myconsumer` Deployment:

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
  # Specifies the controller to scale (reference to a Deployment or StatefulSet).
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

### Scaling based on external metrics

The `mymetric` metric is sourced from an external system (e.g., SQS). Scaling is triggered when the metric value exceeds 100:

```yaml
kind: HorizontalPodAutoscaler
apiVersion: autoscaling/v2
metadata:
  name: myhpa
  namespace: mynamespace
spec:
  # Specifies the controller to scale (reference to a Deployment or StatefulSet).
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: myapp
  minReplicas: 1
  maxReplicas: 2
  metrics:
  # Using external metrics for scaling.
  - type: External
    external:
      metric:
        # This is the metric registered in Prometheus as kube_adapter_metric_mymetric,
        # but used here without the 'kube_adapter_metric_' prefix.
        name: mymetric
        selector:
          # Use label selectors to narrow down the metric query.
          matchLabels:
            namespace: mynamespace
            ingress: myingress
      target:
        # Only `type: Value` is supported for External metrics.
        type: Value
        # Scaling is triggered if the metric value exceeds 10.
        value: 10
```
