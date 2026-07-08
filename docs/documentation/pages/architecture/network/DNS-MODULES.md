---
title: DNS modules
permalink: en/architecture/network/dns-modules.html
lang: en
search: dns, coredns, domain names
description: Architecture of the kube-dns and node-local-dns modules in Deckhouse Kubernetes Platform.
relatedLinks:
  - title: "kube-dns module configuration"
    url: /modules/kube-dns/configuration.html
  - title: "node-local-dns module configuration"
    url: /modules/node-local-dns/configuration.html
  - title: "Caching DNS server in a cluster"
    url: /products/kubernetes-platform/documentation/v1/architecture/network/dns-caching.html
---

The [`kube-dns`](/modules/kube-dns/) module provides domain name resolution based on [CoreDNS](https://coredns.io/) in Deckhouse Kubernetes Platform (DKP).

For more details about module configuration and usage examples, refer to [the corresponding documentation section](/modules/kube-dns/configuration.html).

## kube-dns module

### Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`kube-dns`](/modules/kube-dns/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![kube-dns module architecture](../../images/architecture/network/c4-l2-kube-dns.png)

### Module components

The `kube-dns` module consists of the following components:

1. **D8-kube-dns** (Deployment): Main module component implementing a DNS server in the Kubernetes cluster.

   The d8-kube-dns component watches changes in standard Service, EndpointSlice, Namespace, and Pod resources, and periodically requests Node resources. Based on the data it receives, d8-kube-dns updates records in the local object database.

   It consists of the following containers:

   * **coredns**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to coredns metrics. It is an [Open Source project](https://github.com/brancz/kube-rbac-proxy).

1. **D8-kube-dns-sts-pods-hosts-appender-webhook** (Deployment): Optional component that consists of a single **webhook** container.

   The Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module creates this component if the `.spec.settings.clusterDomainAliases` parameter is set in ModuleConfig.

   The component implements a mutating webhook server that adds the **render-etc-hosts-with-cluster-domain-aliases** init container to a pod created by a StatefulSet controller if the `.spec.subdomain` option is specified in the pod spec.

   The init container modifies the `/etc/hosts` file so that the name resolution subsystem works correctly with cluster domain aliases.

### Module interactions

The module interacts with the following components:

* **Kube-apiserver**:

  * Watches standard Service, Endpoint, EndpointSlice, Namespace, Pod, and Node resources.
  * Authorizes requests for metrics.

The following external components interact with the module:

1. **Kube-apiserver**: Modifies Pod resources created by the StatefulSet controller.
1. **Prometheus-main**: Collects module metrics.

## node-local-dns module

The [`node-local-dns`](/modules/node-local-dns/) module provides a caching DNS service and a DNS request forwarding mechanism on each cluster node, reducing the load on `coredns` of the [`kube-dns`](/modules/kube-dns/) module.

Depending on the CNI plugin used, the `node-local-dns` module implements different DNS request forwarding mechanisms:

- When using [`cni-cilium`](/modules/cni-cilium/), Cilium handles DNS request forwarding using the [CiliumLocalRedirectPolicy](https://docs.cilium.io/en/stable/network/kubernetes/local-redirect-policy/#create-cilium-local-redirect-policy-custom-resources) custom resource.
- When using [`cni-flannel`](/modules/cni-flannel/) or [`cni-simple-bridge`](/modules/cni-simple-bridge/), iptables handles DNS request forwarding.

For more details about module configuration and usage examples, refer to [the corresponding documentation section](/modules/node-local-dns/configuration.html).

### When using cni-cilium

#### Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`node-local-dns`](/modules/node-local-dns/) module when using Cilium as the CNI plugin and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![node-local-dns module architecture](../../images/architecture/network/c4-l2-node-local-dns.png)

#### Module components

The `node-local-dns` module consists of the following components:

1. **Node-local-dns** (DaemonSet): Main module component implementing a [caching DNS server](./dns-caching.html) in the Kubernetes cluster.

   The node-local-dns component watches changes in standard EndpointSlice resources and uses them to update the list of DNS servers for forwarding requests.

   It consists of the following containers:

   * **check-linux-kernel**: Init container that checks the Linux kernel version.
   * **coredns**: Main container.
   * **cube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to coredns metrics. It is an [Open Source project](https://github.com/brancz/kube-rbac-proxy).

1. **Stale-dns-connections-cleaner** (DaemonSet): Component that removes stale UDP connections left after the `node-local-dns` Pod is restarted. It consists of a single **stale-dns-connections-cleaner** container.

   {% alert level="warning" %}
   The component has privileged access to the network subsystem of each node. On Linux, this requires the `CAP_NET_ADMIN` capability. This access is required to perform network connection operations at the Linux kernel level.
   {% endalert %}

1. **Safe-updater** (Deployment): Component that provides safe restarts of node-local-dns when the DaemonSet spec changes.

   Safe-updater checks that Cilium is running on the node and is in a correct state, and only then sends a command to delete the `node-local-dns` Pod.

#### Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Watches standard EndpointSlice, DaemonSet, ControllerRevision, and Pod resources.
   * Periodically retrieves Node resources.
   * Deletes the `node-local-dns` Pod when the DaemonSet configuration becomes outdated.
   * Authorizes requests for metrics.

1. **D8-kube-dns**: Executes DNS requests.

The following external components interact with the module:

* **Prometheus-main**: Collects module metrics.

### When using cni-flannel or cni-simple-bridge

#### Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`node-local-dns`](/modules/node-local-dns/) module when using the [`cni-flannel`](/modules/cni-flannel/) or [`cni-simple-bridge`](/modules/cni-simple-bridge/) CNI plugin and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagram:

![node-local-dns module architecture](../../images/architecture/network/c4-l2-node-local-dns-without-cilium.png)

#### Module components

The `node-local-dns` module consists of the following components:

- **Node-local-dns** (DaemonSet): Main module component implementing a [caching DNS server](./dns-caching.html) in the Kubernetes cluster.

   The node-local-dns component watches changes in standard EndpointSlice resources and uses them to update the list of DNS servers for forwarding requests.

   It consists of the following containers:

  * **iptables-wrapper**: Init container that prepares executables required for working with iptables.
  * **coredns**: Main container.
  * **iptables-loop**: Sidecar container that updates iptables rules for DNS request forwarding based on node-local-dns readiness.
  * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to coredns container metrics. It is an [Open Source project](https://github.com/brancz/kube-rbac-proxy).

#### Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Watches standard EndpointSlice resources.
   * Authorizes requests for metrics.

1. **D8-kube-dns**: Executes DNS requests.

The following external components interact with the module:

* **Prometheus-main**: Collects module metrics.
