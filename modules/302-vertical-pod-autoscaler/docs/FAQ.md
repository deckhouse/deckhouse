---
title: "The vertical-pod-autoscaler module: FAQ"
---

## How do I view the Vertical Pod Autoscaler recommendations?

You can view the VPA recommendations after the [VerticalPodAutoscaler](cr.html#verticalpodautoscaler) custom resource is created using the following command:

```shell
d8 k describe vpa my-app-vpa
```

The `status` will have the following parameters:

- `Target` — the optimal amount of resources for the Pod (within the resourcePolicy).
- `Lower Bound` — the minimum recommended amount of resources for the regular operation of the application.
- `Upper Bound` — the maximum recommended amount of resources. Most likely, the resources above this upper bound will never be used by the application.
- `Uncapped Target` — the target computed from resource usage only, without the `resourcePolicy` constraints (`minAllowed`, `maxAllowed`) and without `recommendationFactor`.

## How does Vertical Pod Autoscaler handle limits?

### Example No. 1

The following example shows a VPA object:

```yaml
---
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

The VPA object contains a Pod with the following resources:

```yaml
resources:
  limits:
    cpu: 2
  requests:
    cpu: 1
```

If a container uses all the CPU, and VPA recommends 1.168 CPU for that container, then the ratio between requests and limits will be 100%.
In this case, when recreating the Pod, VPA will modify it and set the following resources:

```yaml
resources:
  limits:
    cpu: 2336m
  requests:
    cpu: 1168m
```

### Example No. 2

The following example shows a VPA object:

```yaml
---
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

The VPA object contains a pod with resources:

```yaml
resources:
  limits:
    cpu: 1
  requests:
    cpu: 750m
```

If the request-to-limit ratio is 25% and VPA recommends 1.168 CPU for the container, VPA will change the container resources as follows:

```yaml
resources:
  limits:
    cpu: 1557m
  requests:
    cpu: 1168m
```

If you need to limit the maximum number of resources that can be allocated to container constraints, you should use `maxAllowed` in the VPA object specification or use the [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) of the Kubernetes object.

## How do I make VPA recommend more or less than it measured?

VPA computes the recommendation from the measured resource usage and adds its own safety margin. To make VPA recommend more or less, set the `recommendationFactor` field in the container policy of the [VerticalPodAutoscaler](cr.html#verticalpodautoscaler) object. VPA multiplies the computed recommendation by the specified value. For example, `1.2` increases the recommendation by 20%, and `0.87` decreases it by 13%.

Set the multipliers in the VerticalPodAutoscaler object. A policy with `containerName: "*"` applies to all containers that have no policy of their own:

```yaml
---
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: my-app-vpa
spec:
  targetRef:
    apiVersion: "apps/v1"
    kind: Deployment
    name: my-app
  resourcePolicy:
    containerPolicies:
    - containerName: "*"
      recommendationFactor:
        cpu: "1.2"
        memory: "0.87"
```

The `cpu` and `memory` values are quantities written as strings with at most two decimal places, for example `"1.25"`. The allowed range is greater than zero and up to `10` inclusive. If you omit a key, the recommendation for that resource does not change.

VPA applies the multiplier after its own safety margin and before `minAllowed` and `maxAllowed`, so these constraints still limit the final recommendation. In the `status` of the VPA object, `target`, `lowerBound` and `upperBound` already include the multiplier. `uncappedTarget` shows the target computed without it.

Limits follow requests proportionally, as described in the section [How does Vertical Pod Autoscaler handle limits?](#how-does-vertical-pod-autoscaler-handle-limits).

{% alert level="warning" %}
A `memory` multiplier below `1` increases the risk of OOM kills, because it reduces the headroom VPA keeps above the measured usage.
{% endalert %}

Changing a multiplier changes the recommendation, so the `Updater` component can update the Pods of the application according to its usual rules.

For a DaemonSet with `spec.scope`, VPA stores the recommendations in `status.groups` in a compact form that contains only `target` for each container. The value without the multiplier is not shown there.
