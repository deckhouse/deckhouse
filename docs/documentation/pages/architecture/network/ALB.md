---
title: Alb module
permalink: en/architecture/network/alb.html
search: alb, application load balancer, gateway api
description: Architecture of the alb module in Deckhouse Platform.
---

The [`alb`](/modules/alb/) module implements an application load balancer (ALB, Application Load Balancer) in Deckhouse Platform (DP) and lets you publish applications using the [Kubernetes Gateway API](https://gateway-api.sigs.k8s.io/). It deploys and configures the infrastructure for accepting and routing external requests, and validates the user's Gateway API configuration.

The module works with the following custom resources of the `network.deckhouse.io` API group:

- [ClusterALBInstance](/modules/alb/cr.html#clusteralbinstance): Defines a cluster-wide ALB (data plane) instance, used for shared or platform-level Gateways.
- [ALBInstance](/modules/alb/cr.html#albinstance): Defines an ALB instance within a user namespace, used for application- or project-level Gateways.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/alb/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`alb`](/modules/alb/) module and its interactions with other components of DP are shown in the following diagram:

![Alb module architecture](../../images/architecture/network/c4-l2-alb.svg)

## Module components

The module consists of the following components:

1. **Proxy-configurator** (Deployment): The Gateway API control-plane component, built from Istio Pilot (istiod) configured to work with Gateway API only (sidecar injection and istiod's built-in Gateway API infrastructure-provisioning mechanism are disabled).

   The component serves configuration to the Envoy proxies over the xDS protocol and issues certificates for them via a built-in CA server. It also validates and updates the status of Gateway, HTTPRoute, GRPCRoute, TCPRoute, UDPRoute, TLSRoute, and ListenerSet resources.

   It consists of a single **proxy-configurator** container.

1. **Gateway-controller** (Deployment): the module's central controller, which performs the following actions:

   * Manages the ClusterALBInstance and ALBInstance custom resources.
   * Installs Gateway API CRDs (`*.gateway.networking.k8s.io`).
   * Implements the Gateway API controller for the `d8-alb` GatewayClass and creates a Gateway for every instance.
   * Serves the admission webhooks that validate them.
   * Creates temporary Ingress objects for the cert-manager HTTP-01 challenge on top of HTTPRoute when migrating from [`ingress-nginx`](/modules/ingress-nginx/).
   * Creates and removes the proxy and geoproxy components for every ClusterALBInstance/ALBInstance.

   It consists of the following containers:

   * **gateway-controller**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to gateway-controller metrics and the graph API.

1. **Proxy** (Deployment or DaemonSet): an Envoy data-plane instance that accepts and routes external traffic according to configuration received from proxy-configurator over the xDS protocol.

   The gateway-controller creates this component dynamically (not via a Helm template) for every ClusterALBInstance or ALBInstance custom resource. The workload kind depends on the resource type: a ClusterALBInstance is deployed as a DaemonSet, an ALBInstance as a Deployment behind a Service.

   It consists of the following containers:

   * **geo-downloader-init**: Optional init container that downloads the GeoIP database before Envoy starts.
   * **geo-downloader**: Optional sidecar container that periodically refreshes the local copy of the GeoIP database.
   * **proxy**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to the main container's metrics.

   The gateway-controller adds the first two containers only if the [`spec.geoIP`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-geoip) parameter is set on the ClusterALBInstance/ALBInstance resource.

1. **Geoproxy** (StatefulSet): a caching proxy server for the proxy component that provides fast access to the GeoIP database downloaded from the MaxMind provider. The component also provides access to already-downloaded databases in clusters without internet access, which improves the stability of the Gateway API controller's work with GeoIP databases.

   The component provides the following benefits:

   * Saves MaxMind licenses (databases are downloaded from a single point once a day).
   * Persistent data storage (restarting components does not cause repeated requests to MaxMind servers).
   * The ability to specify your own mirror for downloading the databases.

   The gateway-controller creates this component programmatically for every ClusterALBInstance/ALBInstance, only if the [`spec.geoIP.licenseKeySecretRef`](/modules/alb/cr.html#clusteralbinstance-v1alpha1-spec-geoip-licensekeysecretref) parameter — a reference to the Secret holding the MaxMind license key — is set on the resource.

   It consists of a single **geoproxy** container.

1. **Module-cleanup-waiter** (Job): a Helm `pre-delete` hook of the module, launched by the Deckhouse controller before the [`alb`](/modules/alb/) module is removed. It waits for the gateway-controller to finish cleaning up the module's resources, and only then allows the release removal to complete. It does not take part in the module's normal operation.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Installs resources of the `*.gateway.networking.k8s.io` API group.
   * Manages DaemonSet, Deployment, StatefulSet, Service, Secret, and ConfigMap resources.
   * Reads and updates Node and Namespace resources.
   * Manages the ClusterALBInstance and ALBInstance custom resources, Ingress resources, and Gateway API resources (the `*.gateway.networking.k8s.io` API group).
   * Authorizes requests for gateway-controller metrics and the graph API, and for proxy metrics.

1. **GeoIP data source** (MaxMind provider or mirror): Downloads the GeoIP database.

The following external components interact with the module:

1. **Kube-apiserver**: Calls validation for the ClusterALBInstance and ALBInstance custom resources and for Gateway API resources (the `*.gateway.networking.k8s.io` API group).

1. **Prometheus-main**:

   * Collects gateway-controller metrics.
   * Collects proxy metrics.

1. **Load balancer**: Balances HTTP/HTTPS traffic between instances of the proxy component.

1. **[Deckhouse web UI](/modules/console/)**: Requests the Gateway API resource relationship graph for visualization.
