---
title: "Vertical pod autoscaling"
permalink: en/user/configuration/app-scaling/vpa.html
---

## How Vertical Pod Autoscaler (VPA) works

The Vertical Pod Autoscaler (VPA) automates container resource management and can significantly improve application performance. VPA is especially useful when it's difficult to estimate resource needs in advance. When VPA is used in an appropriate operating mode, it sets requested resources based on actual usage data collected [from the monitoring system](../../monitoring/). You can also configure it to only provide recommendations without applying changes automatically.

## How VPA interacts with limits

VPA manages the container's resource **requests**, but it does not manage **limits** unless explicitly configured to do so.

VPA calculates recommended values based on resource usage by the container. This behavior can affect the ratio between requests and limits:

- If requests and limits are equal, VPA will only update the requests, leaving limits unchanged.
- If limits are not specified, VPA will only update requests.
- If limits are set but not controlled by VPA, the ratio between requests and limits may shift.

1. Example 1. In the cluster, we have:

   - A VPA object:

     ```yaml
     apiVersion: autoscaling.k8s.io/v1
     kind: VerticalPodAutoscaler
     metadata:
       name: test2
     spec:
       targetRef:
         apiVersion: "apps/v1"
         kind: Deployment
         name: test2
       updatePolicy:
         updateMode: "Initial"
     ```

   - A pod with specified resources:

     ```yaml
     resources:
     limits:
       cpu: 2
     requests:
       cpu: 1
     ```

     If the container consumes 1 CPU, VPA will recommend 1.168 CPU. In this case, the ratio between requests and limits is 100% (since the request is managed, but the limit remains unchanged). When the pod is recreated, VPA will update the resources as follows:

     ```yaml
     resources:
     limits:
       cpu: 2336m
     requests:
       cpu: 1168m
     ```

1. Example 2. The cluster contains:

   - A VPA object:

     ```yaml
     apiVersion: autoscaling.k8s.io/v1
     kind: VerticalPodAutoscaler
     metadata:
       name: test2
     spec:
       targetRef:
         apiVersion: "apps/v1"
         kind: Deployment
         name: test2
       updatePolicy:
         updateMode: "Initial"
     ```

   - A pod with specified resources:

     ```yaml
     resources:
     limits:
       cpu: 1
     requests:
       cpu: 750m
     ```

    In this case, the ratio between requests and limits will be 25%. If VPA recommends 1.168 CPU, the container's resources will be adjusted to:

     ```yaml
     resources:
       limits:
         cpu: 1557m
       requests:
         cpu: 1168m
     ```

If resources are not limited, VPA might assign excessively high resource values, which can cause issues.

To prevent this, you can:

- Use the `maxAllowed` parameter in the VPA specification:

  ```yaml
  apiVersion: autoscaling.k8s.io/v1
  kind: VerticalPodAutoscaler
  metadata:
    name: my-app-vpa
  spec:
    targetRef:
      apiVersion: "apps/v1"
      kind: Deployment
      name: my-app
    updatePolicy:
      updateMode: "Auto"
    resourcePolicy:
      containerPolicies:
      - containerName: hamster
        minAllowed:
          memory: 100Mi
          cpu: 120m
        maxAllowed:
          memory: 300Mi
          cpu: 350m
        mode: Auto
  ```

- Configure a [`LimitRange`](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) in the cluster:

  ```yaml
  apiVersion: v1
  kind: LimitRange
  metadata:
    name: my-app-limits
  spec:
    limits:
    - default:
        cpu: 2
        memory: 4Gi
      defaultRequest:
        cpu: 500m
        memory: 256Mi
      type: Container
  ```

## Monitoring VPA in Grafana

To efficiently manage resources using the Vertical Pod Autoscaler (VPA), it is recommended to use [Grafana dashboards](../../web/grafana.html#working-with-dashboards). These dashboards allow you to track the current status of VPA, its configuration, and the percentage of pods on which it is active.

Grafana provides several levels of detail for VPA-related information. Key dashboards include:

- Main / Namespace — Displays general VPA usage per namespace.
- Main / Namespace / Controller — Provides VPA metrics for specific controllers.
- Main / Namespace / Controller / Pod — The most granular view, showing data for each individual pod.

Key columns to monitor:

- VPA type — Shows the current value of `updatePolicy.updateMode`, which defines the VPA operating mode. This field appears in the following dashboards:
  - Main / Namespace
  - Main / Namespace / Controller
  - Main / Namespace / Controller / Pod

- VPA % (Percentage of pods with VPA enabled) — Displays the percentage of pods within a namespace that have VPA enabled. This helps quickly assess how much of the cluster is covered by automatic resource scaling via VPA.

## VPA configuration examples

1. Example of a minimal `VerticalPodAutoscaler` resource:

   ```yaml
   apiVersion: autoscaling.k8s.io/v1
   kind: VerticalPodAutoscaler
   metadata:
     name: my-app-vpa
   spec:
     targetRef:
       apiVersion: "apps/v1"
       kind: StatefulSet
       name: my-app
   ```

1. Example of a full VerticalPodAutoscaler resource:

   ```yaml
   apiVersion: autoscaling.k8s.io/v1
   kind: VerticalPodAutoscaler
   metadata:
     name: my-app-vpa
   spec:
     targetRef:
       apiVersion: "apps/v1"
       kind: Deployment
       name: my-app
     updatePolicy:
       updateMode: "Auto"
     resourcePolicy:
       containerPolicies:
       - containerName: hamster
         minAllowed:
           memory: 100Mi
           cpu: 120m
         maxAllowed:
           memory: 300Mi
           cpu: 350m
         mode: Auto
    ```
