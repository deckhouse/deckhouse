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
- `Uncapped Target` — the recommended amount of resources based on the latest metrics (the history of resource usage is ignored).

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
