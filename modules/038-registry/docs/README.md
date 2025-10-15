---
title: "Registry Module"
description: "Configuration management of DKP component registry and organization of an internal container registry."
---

## Description

The module implements the internal container image registry.

The internal registry allows for optimizing the downloading and storage of images, as well as helping to ensure availability and fault tolerance for Deckhouse Kubernetes Platform.

The module can operate in the following modes:

- `Direct` — enables the internal container image registry. Access to the internal registry is performed via the fixed address `registry.d8-system.svc:5001/system/deckhouse`. This fixed address allows Deckhouse images to avoid being re-downloaded and components to avoid being restarted when registry parameters change. Switching between modes and registries is done through the `deckhouse` ModuleConfig. The switching process is automatic — see the [usage examples](examples.html) for more information.
- `Unmanaged` — operation without using the internal registry. Access within the cluster is performed via an address that can be [set during the cluster installation](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#initconfiguration-deckhouse-imagesrepo) or [changed in a deployed cluster](/products/kubernetes-platform/documentation/v1/admin/configuration/registry/third-party.html).

## Restrictions and features of using the module

The `registry` module has a number of limitations and features related to installation, operating conditions, and mode switching.

### Cluster installation limitations

Bootstrapping a DKP cluster with `Direct` mode enabled is not supported. The cluster is deployed with settings for `Unmanaged` mode.

### Operating conditions restrictions

The module works under the following conditions:

- If CRI containerd or containerd v2 is used on the cluster nodes. To configure CRI, refer to the [ClusterConfiguration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#clusterconfiguration-defaultcri) configuration.
- The cluster is fully managed by DKP. The module will not work in Managed Kubernetes clusters.

### Mode switching restrictions

Mode switching restrictions are as follows:

- Switching to `Direct` mode is possible if there are no user registry configurations on the nodes. For more details, see the [Registry Module: FAQ](./faq.html) section.
- Switching to `Unmanaged` mode is only available from `Direct` mode.
- In `Unmanaged` mode, changing registry settings is not supported. To change settings, you need to switch to `Direct` mode, make the necessary changes, and then switch back to `Unmanaged` mode.

## Direct Mode Architecture

In Direct mode, registry requests are processed directly, without intermediate caching.

CRI requests to the registry are redirected based on its configuration, which is defined in the `containerd` configuration.

For components such as `operator-trivy`, `image-availability-exporter`, `deckhouse-controller`, and others that access the registry directly, requests will go through the in-cluster proxy located on the master nodes.

<!--- Source: mermaid code from docs/internal/DIRECT.md --->
![direct](images/direct-en.png)

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
