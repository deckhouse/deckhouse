---
title: "Customizing istio-proxy sidecar resource management"
permalink: en/user/network/istio-sidecar-resource-management.html
---

When using the [`istio`](../../modules/istio/) module in a cluster, you can manage the resources allocated to istio-proxy sidecars to workloads. Annotations are used for this purpose.

## Supported annotations

To override global resource limits for istio-proxy sidecars, annotations are supported in individual workloads:

|Annotation                          | Description                  | Example Value |
|-------------------------------------|-----------------------------|---------------|
| `sidecar.istio.io/proxyCPU`         | CPU request for sidecar     | `200m`        |
| `sidecar.istio.io/proxyCPULimit`    | CPU limit for sidecar       | `"1"`         |
| `sidecar.istio.io/proxyMemory`      | Memory request for sidecar  | `128Mi`       |
| `sidecar.istio.io/proxyMemoryLimit` | Memory limit for sidecar    | `512Mi`       |

{% alert level="warning" %}
All annotations from the table must be specified in the workload manifest at the same time. Partial configuration is not supported.
{% endalert %}

## Configuration Examples

For Deployments:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
# ...
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/proxyCPU: 200m
        sidecar.istio.io/proxyCPULimit: "1"
        sidecar.istio.io/proxyMemory: 128Mi
        sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... rest of your deployment spec
```

For ReplicaSets:

```yaml
apiVersion: apps/v1
kind: ReplicaSet
metadata:
# ...
spec:
  template:
    metadata:
      annotations:
        sidecar.istio.io/proxyCPU: 200m
        sidecar.istio.io/proxyCPULimit: "1"
        sidecar.istio.io/proxyMemory: 128Mi
        sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... rest of your deployment spec
```

For Pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    sidecar.istio.io/proxyCPU: 200m
    sidecar.istio.io/proxyCPULimit: "1"
    sidecar.istio.io/proxyMemory: 128Mi
    sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... rest of your pod spec
```
