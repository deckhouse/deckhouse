---
title: "Module registry"
description: ""
---

## Description

The module implements an internal container image registry. It allows you to use a local registry to optimize image downloading and storage, as well as to provide high availability and fault tolerance.

The module supports several operating modes, allowing it to be adapted to different usage scenarios.

The module operates in `Direct`, `Proxy`, and `Local` modes (currently, only the `Direct` mode is supported).

### Direct mode

In this mode, requests to the registry are processed directly, without intermediate caching.

{% alert level="info" %}
To use the `Direct` mode, you must use the `Containerd` or `ContainerdV2` CRI on all cluster nodes.
{% endalert %}

Redirecting registry requests from the CRI is done using its settings, which are specified in the `containerd` configuration.

For components that access the registry directly, such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and several others, requests will go through the In-Cluster Proxy located on the control plane nodes.

```mermaid
---
title: Direct Mode
---
flowchart TD
subgraph Cluster["Deckhouse Kubernetes Cluster"]

subgraph Node1["Node 1"]
kubelet[Kubelet]
containerd[Containerd]
end

subgraph Node2["Node 2"]
kubelet2[Kubelet]
containerd2[Containerd]
end

subgraph Node3["Node 3"]
kubelet3[Kubelet]
containerd3[Containerd]
end


kubelet --> containerd
kubelet2 --> containerd2
kubelet3 --> containerd3

subgraph InCluster["Cluster Components"]
operator[operator-trivy]
controller[deckhouse-controller]
exporter[image-availability-exporter]
registrySVC["In-cluster Proxy **(registry.d8-system.svc:5001)**"]
end

operator --> registrySVC 
controller --> registrySVC
exporter --> registrySVC

end

registryRewritten[("**registry.deckhouse.ru**")]


registrySVC --> registryRewritten

containerd -. "**REWRITE** registry.d8-system.svc:5001" .-> registryRewritten
containerd2 -. "**REWRITE** registry.d8-system.svc:5001" .-> registryRewritten
containerd3 -. "**REWRITE** registry.d8-system.svc:5001" .-> registryRewritten
```
