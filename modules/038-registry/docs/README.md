---
title: "Registry Module"
description: ""
---

## Description

The registry module is a component that implements the internal container image registry of Deckhouse.

The internal registry allows for optimizing the downloading and storage of images, and also ensures high availability and fault tolerance for Deckhouse Kubernetes Platform.

The module can operate in the following modes:

- `Direct` — enables the internal container image registry. Access to the internal registry is performed via the fixed address `registry.d8-system.svc:5001/system/deckhouse`. This fixed address allows Deckhouse images to avoid being re-downloaded and components to avoid being restarted when registry parameters change. Switching between modes and external registries is done through the `Deckhouse` `ModuleConfig`. The switching process is automatic — see the [usage examples](./examples.html) for more information.

- `Unmanaged` — operates without using the internal registry. In this mode, access inside the cluster is performed via the address specified during cluster bootstrap, or via the [`helper change-registry`](/products/kubernetes-platform/documentation/v1/deckhouse-faq.html#how-do-i-switch-a-running-deckhouse-cluster-to-use-a-third-party-registry) tool.

{% alert level="info" %}
- The `Direct` mode requires using the `Containerd` or `Containerd V2` CRI on all cluster nodes. For CRI setup, refer to the [`ClusterConfiguration`](/products/kubernetes-platform/documentation/v1/installing/configuration.html#clusterconfiguration).
{% endalert %}

### Direct Mode Architecture

In Direct mode, registry requests are processed directly, without intermediate caching.

CRI requests to the registry are redirected based on its configuration, which is defined in the `containerd` configuration.

For components such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others that access the registry directly, requests will go through the in-cluster proxy located on the control plane nodes.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](../../images/registry-module/direct-en.png)

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
