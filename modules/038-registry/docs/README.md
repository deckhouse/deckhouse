---
title: "Registry Module"
description: ""
---

## Description

The registry module is a component that implements an internal container image registry for Deckhouse.  
The internal container image registry helps optimize image pulling and storage, providing high availability and fault tolerance for Deckhouse.  

The module supports several modes of operation: `Direct`, `Proxy`, `Local`, and `Unmanaged` (currently, only `Direct` and `Unmanaged` modes are supported).  

The `Direct`, `Proxy`, and `Local` modes (referred to as `Managed` modes) implement internal container image registry functionality. These modes allow the registry to be adapted to different usage scenarios.  
In these modes, access to the internal registry always occurs via the fixed address:  
`registry.d8-system.svc:5001/system/deckhouse`
This fixed address eliminates the need to re-download images and restart components when registry parameters change or the mode is switched.

The `Unmanaged` mode allows operating without the internal registry. Within the cluster, image access occurs via the address specified during bootstrap or updated using the `helper change-registry` utility.

{% alert level="info" %}
To use the `Direct`, `Proxy`, or `Local` modes, the `Containerd` or `ContainerdV2` CRI must be used on all nodes of the cluster.  
For CRI setup, refer to the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration) documentation.
{% endalert %}

### Direct Mode

In Direct mode, registry requests are handled directly without intermediate caching.  
Requests from the CRI to the registry are redirected using registry settings specified in the containerd configuration.  

For components that interact with the registry directly, such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others, requests are routed through an In-Cluster Proxy located on the control plane nodes.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](../../images/registry-module/direct.png)

<!-- ### Proxy Mode
This mode allows the registry to act as an intermediate proxy server between the client and the remote registry, optimizing access to frequently used images and reducing network load.
The caching proxy registry runs as static pods on control plane nodes. To ensure high availability, a load balancer is deployed on each cluster node.
Registry access from the CRI is performed through the load balancer, with the corresponding configuration set in containerd.
For components that access the registry directly, such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others, requests will also go through the caching proxy registry.
-->

<!-- ### Local Mode
This mode enables the creation of a local registry copy inside the cluster. Images from the remote registry are fully replicated to local storage.
Operation is similar to the caching proxy. The local registry also runs as static pods on control plane nodes. A per-node load balancer is used to ensure availability.
CRI access to the local registry is set up via the load balancer and configured in containerd.
Components that access the registry directly, such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others, will go to the local registry.
Populating the local registry is handled using the d8 tool.
-->
