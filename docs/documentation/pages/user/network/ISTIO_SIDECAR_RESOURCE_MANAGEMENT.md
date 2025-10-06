---
title: "Configuring resources for istio-proxy sidecars"
permalink: en/user/network/istio-sidecar-resource-management.html
---

When using the [`istio`](/modules/istio/) module in a cluster,
you can manage the resources allocated for istio-proxy sidecars in specific workloads.
Annotations are used for this purpose.

## Supported annotations

To override global resource limits for istio-proxy sidecars, the following annotations are supported in individual workloads:

|Annotation                          | Description                  | Example value |
|-------------------------------------|-----------------------------|---------------|
| `sidecar.istio.io/proxyCPU`         | CPU request for a sidecar     | `200m`        |
| `sidecar.istio.io/proxyCPULimit`    | CPU limit for a sidecar       | `"1"`         |
| `sidecar.istio.io/proxyMemory`      | Memory request for a sidecar  | `128Mi`       |
| `sidecar.istio.io/proxyMemoryLimit` | Memory limit for a sidecar    | `512Mi`       |

{% alert level="warning" %}
All annotations from the table must be specified in the workload manifest at the same time.
Partial configuration is not supported.
{% endalert %}

## Configuration examples

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
# ... Rest of the manifest.
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
# ... Rest of the manifest.
```

For Pods:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    sidecar.istio.io/proxyCPU: 200m
    sidecar.istio.io/proxyCPULimit: "1"
    sidecar.istio.io/proxyMemory: 128Mi
    sidecar.istio.io/proxyMemoryLimit: 512Mi
# ... Rest of the manifest.
```
