---
title: "Architecture of registry modes"
permalink: en/architecture/registry-modes.html
---

Deckhouse Kubernetes Platform supports several modes of operation with container image storage.

## Direct mode architecture

In Direct mode, registry requests are processed directly, without intermediate caching.

CRI requests to the registry are redirected based on its configuration, which is defined in the `containerd` configuration.

For components such as [operator-trivy](/modules/operator-trivy/), `image-availability-exporter`, `deckhouse-controller`, and others that access the registry directly, requests will go through the in-cluster proxy located on the master nodes.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](../images/registry-module/direct-en.png)

For more information about the `Direct` mode, see the section [Working with container image repositories and editions](../admin/configuration/registry/internal.html).

## Proxy mode architecture

{% alert level="warning" %}
It is recommended to use separate disks for storing registry (`/opt/deckhouse/registry`) and etcd data. Using a single disk may lead to etcd performance degradation during concurrent registry operations.
{% endalert %}

The `Proxy` mode allows the registry to act as an intermediate proxy server between the client and the remote registry.

The caching proxy registry is launched as static pods on control-plane (master) nodes. Cached data is stored on the control-plane (master) nodes in the `/opt/deckhouse/registry` directory.

To ensure high availability of the caching proxy registry, a load balancer is deployed on each cluster node. Access to the proxy registry from CRI goes through this load balancer. The configuration for accessing the load balancer is set in the `containerd` configuration.

For components such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others that access the registry directly, requests will go through the caching proxy registry.

<!--- Source: mermaid code from docs/internal/PROXY.md --->
![direct](../images/registry-module/proxy-en.png)

For more information about the `Proxy` mode, see the section [Working with container image repositories and editions](../admin/configuration/registry/internal.html).

## Local mode architecture

{% alert level="warning" %}
It is recommended to use separate disks for storing registry (`/opt/deckhouse/registry`) and etcd data. Using a single disk may lead to etcd performance degradation during concurrent registry operations.
{% endalert %}

The `Local` mode allows creating a local copy of the registry inside the cluster. Images from the remote registry are fully copied to local storage and synchronized between replicas of the local registry.

The operation of the local registry is identical to that of the caching proxy registry. The local registry is launched as static pods on control-plane (master) nodes. Registry data is stored on the control-plane (master) nodes in the `/opt/deckhouse/registry` directory.

To ensure high availability of the local registry, a load balancer is deployed on each cluster node. Access to the local registry from CRI goes through this load balancer. The configuration for accessing the load balancer is set in the `containerd` configuration.

For components such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others that access the registry directly, requests will go to the local registry.

The local registry is populated using the [`d8`](/products/kubernetes-platform/documentation/v1/cli/d8/) tool with the `d8 mirror push/pull` commands. For more details, see the [«Registry Module: Usage Examples»](/modules/registry/examples.html) section.

<!--- Source: mermaid code from docs/internal/LOCAL.md --->
![direct](../images/registry-module/local-en.png)

For more information about the `Local` mode, see the section [Working with container image repositories and editions](../admin/configuration/registry/internal.html).
