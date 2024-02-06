---
title: "The vertical-pod-autoscaler module: FAQ"
---

## How do I view the Vertical Pod Autoscaler recommendations?

You can view the VPA recommendations after the [VerticalPodAutoscaler](cr.html#verticalpodautoscaler) custom resource is created using the following command:

```shell
kubectl describe vpa my-app-vpa
```

The `status` will have the following parameters:
- `Target` — the optimal amount of resources for the Pod (within the resourcePolicy).
- `Lower Bound` — the minimum recommended amount of resources for the regular operation of the application.
- `Upper Bound` — the maximum recommended amount of resources. Most likely, the resources above this upper bound will never be used by the application.
- `Uncapped Target` — the recommended amount of resources based on the latest metrics (the history of resource usage is ignored).

## How does Vertical Pod Autoscaler handle limits?

### Example No. 1

Suppose we have a VPA object:

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

Also, there is a Pod with the following resource configuration:

```yaml
resources:
  limits:
    cpu: 2
  requests:
    cpu: 1
```

If the container consumes, say, 1 CPU and VPA recommendation for this container is 1.168 CPU, the module will calculate the ration between requests and limits. In our case, the ratio equals 100%.
Thus, VPA will modify the Pod's resource configuration when the Pod is recreated:

```yaml
resources:
  limits:
    cpu: 2336m
  requests:
    cpu: 1168m
```

### Example No. 2

Suppose we have a VPA object:

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

Also, there is a Pod with the following resource configuration:

```yaml
resources:
  limits:
    cpu: 1
  requests:
    cpu: 750m
```

In our case, the ratio of requests and limits is 25%, and the resource configuration of the container will be as follows (given that VPA recommends 1.168 CPU):

```yaml
resources:
  limits:
    cpu: 1557m
  requests:
    cpu: 1168m
```

To limit the maximum amount of resources, set the `maxAllowed` parameter in the object specification or use the [Limit Range](https://kubernetes.io/docs/tasks/administer-cluster/manage-resources/memory-default-namespace/) Kubernetes object.
