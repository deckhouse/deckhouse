---
title: Registry module
permalink: en/architecture/deckhouse/registry.html
search: registry, container registry, direct mode, proxy mode, local mode, unmanaged mode
description: Architecture of the registry module in Deckhouse Kubernetes Platform.
---

The `registry` module manages the registry settings for Deckhouse Kubernetes Platform (DKP) components.

The module can operate in the following modes:

* `Direct`: Provides direct access to an external registry via the fixed address `registry.d8-system.svc:5001/system/deckhouse`. This fixed address prevents Deckhouse images from being re-downloaded and components from being restarted when registry parameters are changed. The fixed address is replaced by the actual address of the external registry when sending requests by configuring `mirroring/rewrite` section in the configuration of the containerd component and by using the internal registry-incluster-proxy component that proxies requests to the external registry.

* `Proxy`: Using an internal caching proxy registry that accesses an external registry, with the caching proxy registry running on control-plane (master) nodes. This mode reduces the number of requests to the external registry by caching images. Cached data is stored on the control-plane (master) nodes. Access to the internal registry is via the fixed address `registry.d8-system.svc:5001/system/deckhouse`, similar to the `Direct` mode.

* `Local`: Using a local internal registry, with the registry running on control-plane (master) nodes. This mode allows the cluster to operate in an isolated environment. All data is stored on the control-plane (master) nodes. Access to the internal registry is via the fixed address `registry.d8-system.svc:5001/system/deckhouse`, similar to the `Direct` and `Proxy` modes.

* `Unmanaged`: Operation without using the internal registry. Access within the cluster is performed directly to the external registry.

For more details about the configuration and usage examples of the module, refer to the [module documentation](/modules/registry/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`registry`](/modules/registry/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

The [`registry`](/modules/registry/) module in `Direct` mode:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Architecture of registry module in Direct mode](../../images/architecture/deckhouse/c4-l2-deckhouse-registry-direct.svg)

The [`registry`](/modules/registry/) module in `Proxy` mode:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Architecture of registry module in Proxy mode](../../images/architecture/deckhouse/c4-l2-deckhouse-registry-proxy.svg)

The [`registry`](/modules/registry/) module in `Local` mode:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_RU --->
![Architecture of registry module in Local mode](../../images/architecture/deckhouse/c4-l2-deckhouse-registry-local.svg)

## Module components

The module consists of the following components:

1. **Registry-incluster-proxy** (`Direct` mode): A container registry based on [Distribution](https://github.com/distribution/distribution). Distribution is an open-source project that provides a framework for storing and distributing container images and other content using the [OCI Distribution Specification](https://github.com/opencontainers/distribution-spec). Registry-inclusion-proxy is installed in the cluster as a Deployment, does not store or cache images, but forwards requests to the external registry specified in [Direct mode settings in `deckhouse` ModuleConfig](/modules/deckhouse/configuration.html#parameters-registry-direct).

   Registry-incluster-proxy consists of the following containers:

   * **distribution**: Main container.
   * **auth**: Sidecar container that implements authentication and authorization when accessing the registry. It is an [open-source project](https://github.com/cesanta/docker_auth).

     The principle of operation:

     1. **Request to the authentication service**. When the client tries to access the registry, distribution returns a `401 Unauthorized` HTTP response with the `WWW-Authenticate` header indicating how to authenticate.

     1. **Getting a token**. The client sends a request to the authentication service via the distribution service (for example, `registry.d8-system.svc:5051/auth`) using the values `service` and `scope` from the header `WWW-Authenticate`. The auth service returns an opaque Bearer token that represents authorized client access.

     1. **Token usage**. After receiving the token, the client resends the original request to distribution, including the token in the `Authorization` header.

     1. **Access verification**. Distribution validates the token and the `claims` contained in it, and then begins the image download session.

1. **Registry-nodeservices** (`Proxy` or `Local` modes): A container registry based on Distribution running on control plane (master) nodes. The registry is launched as a static pod, and its lifecycle is managed by the registry-nodeservices-manager component. The data is stored in the directory `/opt/deckhouse/registry` on the control plane (master) nodes.

   Registry-nodeservices consists of the following containers:

   * **distribution**: Main container.
   * **auth**: Sidecar container for authentication and authorization (described above).
   * **mirrorer** (`Local` mode): Sidecar container that synchronizes images between distribution replicas running on control plane (master) nodes.

1. **Registry-proxy** (`Proxy` or `Local` modes): A load balancer installed on each cluster node that provides high availability for registry. The CRI accesses the registry via the load balancer. The settings to access the load balancer are specified in the containerd configuration.

   It consists of a single container:

   * **registry-proxy**: A container built based on a standard [NGINX](https://github.com/nginx/nginx) image. The registry-proxy-reloader process is also running in the container and restarts the NGINX load balancer processes when its configuration changes.

1. **Registry-nodeservices-manager** (`Proxy` or `Local` modes): A controller that manages the lifecycle of registry-nodeservices components. Registry-nodeservices-manager performs the following operations:

   * Renders the manifest of the registry-nodeservices static pod from templates and saves it in the `/etc/kubernetes` directory on the control plane (master) nodes. [Kubelet](../kubernetes-and-scheduling/kubelet.html) detects created manifests and runs registry-nodeservices static pod.
   * Saves configuration files necessary for the operation of registry-nodeservices components in the `/etc/kubernetes/registry` directory on control plane (master) nodes.
   * Deletes the static pod manifest in the `/etc/kubernetes` directory and configuration files in the `/etc/kubernetes/registry` directory, thereby deleting the local container registry, in case the module switches to the `Direct` or `Unmanaged` mode.

   Registry-nodeservices-manager consists of the following containers:

   * **registry-nodeservices-manager**: Main container.

   * A set of sidecar containers used to pre-pull images of registry-nodeservices components. These containers remain paused and serve only as image holders:

     * **image-holder-auth**
     * **image-holder-distribution**
     * **image-holder-mirrorer**

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Checks that the current node is the master.
   * Checks that all containers of the registry-nodeservices static pod are pulled and status of registry-nodeservices pod is `Ready`.
   * Applies configuration changes from `registry-node-config-<Node Name>` Secret to the registry-nodeservices static pod.

1. **External container registry**: The module forwards internal requests to the external container registry.

The following external components interact with the module:

1. **Containerd**: Sends requests to the registry to pull images used to create containers.

1. **Deckhouse-controller**: Sends requests to the registry to pull images used to install modules.

1. **Operator-trivy (as well as other components that access the registry directly)**: Sends requests to the registry to download images, for example, to perform security tests (in case of operator-trivy).
