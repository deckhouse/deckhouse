---
title: Istio module
permalink: en/architecture/network/istio.html
search: istio, service mesh, ambient, federation, multicluster, sidecar
description: Architecture of the istio module in Deckhouse Kubernetes Platform.
---

The [`istio`](/modules/istio/) module implements a Service Mesh based on [Istio](https://istio.io/) for centralized management of network traffic in Deckhouse Kubernetes Platform (DKP). The module provides mTLS (Mutual Transport Layer Security), request authorization, traffic routing, load balancing, and observability of interactions between applications.

The [`istio`](/modules/istio/) module allows several Istio versions to run at the same time. The module's [`globalVersion`](/modules/istio/configuration.html#parameters-globalversion) parameter specifies which Istio version is used by default for namespaces labeled `istio-injection: enabled`. If a namespace needs to use an Istio version other than the default one, it is labeled with the label corresponding to the Istio revision instead, for example, `istio.io/rev: v1x27`.

The module works with the following custom resources.

Resources managed directly by the module (the `deckhouse.io` group):

* [IngressIstioController](/modules/istio/cr.html#ingressistiocontroller): Describes an Istio ingress gateway instance serving the selected gateway class.
* [IstioFederation](/modules/istio/cr.html#istiofederation): Marks one or more remote clusters as trusted for mesh federation (Enterprise Edition).
* [IstioMulticluster](/modules/istio/cr.html#istiomulticluster): Marks one or more remote clusters as trusted for a multicluster configuration (Enterprise Edition).
* [WaypointInstance](/modules/istio/cr.html#waypointinstance): Describes a waypoint ambient proxy created by the waypoint-controller component (Enterprise Edition).

The module also installs and uses the standard [Istio](https://istio.io/) custom resources (the `networking.istio.io`, `security.istio.io`, `telemetry.istio.io`, and `extensions.istio.io` groups — VirtualService, DestinationRule, Gateway, PeerAuthentication, and others). For more details, see the [Istio custom resource reference](/modules/istio/istio-cr.html).

{% alert level="warning" %}
DKP only supports operator-based management for Istio version 1.25. All later versions run without the operator.
Istio 1.25 is deprecated and will be removed in a future update.
{% endalert %}

If an Istio version with operator support is requested, the following [Sail Operator](https://github.com/istio-ecosystem/sail-operator) custom resources of the `sailoperator.io` group are installed and used:

* Istio: Represents an Istio Service Mesh deployment consisting of one or more control planes.
* IstioRevision: Represents a single revision of the Istio control plane.

The set of module components and its architecture depend on the DKP edition. The Enterprise Edition (EE) adds cross-cluster service mesh federation, periodic service mesh configuration analysis, and support for [Istio ambient mode](https://istio.io/latest/docs/ambient/overview/).

For more details about module configuration, refer to the [corresponding documentation section](/modules/istio/).

## Module architecture

The Level 2 C4 architecture of the [`istio`](/modules/istio/) module and its interactions with other components of Deckhouse Kubernetes Platform (DKP) are shown in the following diagrams:

{% alert level="info" %}
The following simplifications are made in the diagrams:

* The diagrams show containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagrams.
{% endalert %}

* Base module functionality (control plane, CNI, ingress gateway, Kiali, config-analyzer):

  ![Istio module architecture](../../images/architecture/network/c4-l2-istio.en.svg)

* Ambient mode (ztunnel, waypoint-controller; available only in the Enterprise Edition, disabled by default — the diagram shows only the differences from the base configuration):

  ![Istio module architecture in ambient mode](../../images/architecture/network/c4-l2-istio-ambient.en.svg)

* Federation and multicluster configuration (available only in the Enterprise Edition, disabled by default — the diagram shows only the differences from the base configuration):

  ![Istio module architecture in federation/multicluster configuration](../../images/architecture/network/c4-l2-istio-multicluster.en.svg)

## Module components

The module consists of the following components:

1. **Operator-&lt;VERSION&gt;** (Deployment): an implementation of the [Sail Operator](https://github.com/istio-ecosystem/sail-operator) that manages the Istio control plane lifecycle. The component is responsible for installing all the resources required for a given control plane version to work.

   DKP only supports the operator for Istio version 1.25.

   The component watches the Istio and IstioRevision custom resources and uses them to create the `istiod-<VERSION>` Deployment, a Service, and a ConfigMap. Webhook management for validation and mutation is deliberately disabled in the operator; it is handled by the Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module when the module's Helm chart is applied.

   It consists of a single container:

   * **operator**: Main container.

1. **Istiod-&lt;VERSION&gt;** (Deployment): the Istio control plane component that:

   * Distributes sidecar proxy routing configuration via [xDS](https://github.com/cncf/xds).
   * Issues certificates for workloads managed by Istio.
   * Validates user application pods and mutates them to inject sidecar containers, via [Validating/Mutating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/).
   * Validates custom resources of the `*.istio.io` API groups via Validating Admission Controllers.

   For Istio 1.25 (deprecated and scheduled for removal), the component is created and managed by the operator-&lt;VERSION&gt; component through the Istio and IstioRevision custom resources. For [other Istio versions supported in DKP](/modules/istio/#compatibility-table-for-supported-versions), it is deployed directly by the module's Helm chart.

   It consists of a single container:

   * **discovery**: Main container.

1. **Ingress-gateway-controller-&lt;NAME&gt;** (DaemonSet): controller that handles incoming mesh traffic from applications. Created by the Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module for each [IngressIstioController](/modules/istio/cr.html#ingressistiocontroller) custom resource. The `<NAME>` placeholder is the name of the IngressIstioController resource.

   It consists of a single container:

   * **istio-proxy**: Main container based on the open-source [Envoy](https://github.com/envoyproxy/envoy) project, which handles incoming traffic and receives configuration via xDS from the istiod controller.

1. **Kiali** (Deployment): [Kiali](https://kiali.io/) web interface for managing and observing Istio resources and user services running under Istio, allowing you to:

   * Visualize connections between services.
   * Diagnose problematic connections between services.
   * Diagnose the state of the Istio control plane.

   It consists of the following containers:

   * **kiali**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to the Kiali web interface.

   Authentication of Kiali web interface users is performed by the [`user-authn`](/modules/user-authn/) module through a dedicated dex-authenticator.

1. **Istio-config-analyzer-&lt;VERSION&gt;** (Deployment): component that performs periodic service mesh configuration analysis (`istioctl analyze`) and exports the analysis results as Prometheus metrics.

   The component is created by the Deckhouse controller of the [`deckhouse`](/modules/deckhouse/) module for each Istio revision if the [`.settings.configAnalysis.enabled`](/modules/istio/configuration.html#parameters-configanalysis-enabled) module parameter is set to `true` (enabled by default).

   It consists of the following containers:

   * **istio-config-analyzer**: Main container.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to metrics.

1. **Istio-cni-node** (DaemonSet): Istio component that installs the CNI plugin on each cluster node and, in Istio ambient mode, sets up traffic interception for pods.

   The component prepares the `istio-cni` binary and appends it as an additional plugin to the first CNI config found in the `/etc/cni/net.d/` directory on each cluster node. In the standard DKP configuration this is the `05-cilium.conflist` file, created by the Cilium CNI plugin of the [`cni-cilium`](/modules/cni-cilium/) module. When creating each pod, kubelet (via containerd) calls both CNI plugins in sequence — first cilium, then istio-cni. The result produced by the first plugin is passed to the second.

   In Istio ambient mode, the component handles API requests from the `istio-cni` CNI plugin and configures routing to the ztunnel component.

   The component is created by the Deckhouse controller if the [`.settings.dataPlane.trafficRedirectionSetupMode`](/modules/istio/configuration.html#parameters-dataplane-trafficredirectionsetupmode) module parameter is set to `CNIPlugin` (the default is `InitContainer`).

   It consists of the following containers:

   * **install-cni**: Main container that installs and configures the CNI plugin on the node.
   * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to install-cni metrics.

1. **Istio-cni**: binary invoked by containerd, which receives the command (for example, ADD when a container starts and DEL when it is removed) and other parameters via environment variables, per the [CNI specification](https://www.cni.dev/docs/spec/#cni-operations), and the JSON configuration via stdin.

   On each invocation, istio-cni performs the following actions:

   * Processes the input from containerd: the JSON configuration, and the pod's name and namespace.
   * Retrieves pod and namespace information from kube-apiserver.
   * Stops processing if the namespace is in `exclude_namespaces` (a configuration parameter in the `cni-config` ConfigMap).
   * In ambient mode, checks whether the pod is enabled for ambient via labels and, if so, notifies the istio-cni-node component via a Unix socket.
   * In sidecar mode, checks that the pod has no `istio-init` container, has an `istio-proxy` container, and checks annotations; if the conditions are met, it enters the pod's network namespace (netns) and sets up interception of inbound and outbound traffic by running `iptables` or `nftables` commands.

1. **Ztunnel** (DaemonSet): Istio ambient mode component that provides the L4 data plane (mTLS, L4-level authorization) without adding a sidecar to the user's application. Runs on every cluster node.

   The component is created by the Deckhouse controller if the [`.settings.ambient.enabled`](/modules/istio/configuration.html#parameters-ambient-enabled) module parameter is set to `true` (disabled by default).

   It consists of a single container:

   * **istio-proxy**: Main container that receives configuration via xDS from the istiod controller.

1. **Waypoint-controller** (Deployment): controller for the ambient mode L7 layer. The controller manages the [WaypointInstance](/modules/istio/cr.html#waypointinstance) custom resource and creates or updates the corresponding `waypoint-<NAME>` Deployment.

   Created under the same conditions as ztunnel.

   It consists of a single container:

   * **waypoint-controller**: Main container.

1. **Waypoint-&lt;NAME&gt;** (Deployment): proxying service that handles L7 traffic (HTTP routing and authorization) for the services or workloads of the given namespace. Created dynamically by the waypoint-controller component for each WaypointInstance resource.

   It consists of a single container:

   * **istio-proxy**: Main container that receives configuration via xDS from the istiod controller.

1. **Metadata-exporter** (Deployment): component that provides public cluster metadata (the CA root certificate, public keys, endpoint addresses) to remote clusters for configuring cross-cluster interaction ([federation](/modules/istio/#federation) or [multicluster](/modules/istio/#multicluster)).

    The component is created by the Deckhouse controller if the [`.settings.federation.enabled`](/modules/istio/configuration.html#parameters-federation-enabled) or the [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) module parameter is enabled (both disabled by default).

    It consists of the following containers:

    * **metadata-exporter**: Main container.
    * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to metrics.

1. **Alliance-healthcheck** (Deployment): component that checks the availability of the mesh connection with remote clusters and updates the status of the IstioFederation and IstioMulticluster custom resources.

    Created under the same conditions as metadata-exporter.

    It consists of a single container:

    * **healthcheck**: Main container.

1. **Ingressgateway** (DaemonSet): component that accepts mesh traffic from remote clusters via mTLS with SNI passthrough. A separate component from ingress-gateway-controller-&lt;NAME&gt;, dedicated exclusively to federation and multicluster traffic.

    The component is created by the Deckhouse controller if either:

    * The [`.settings.federation.enabled`](/modules/istio/configuration.html#parameters-federation-enabled) module parameter is enabled.
    * The [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) module parameter and the [`.spec.enableIngressGateway`](/modules/istio/cr.html#istiomulticluster-v1alpha1-spec-enableingressgateway) parameter of the IstioMulticluster custom resource are both enabled.

    It consists of a single container:

    * **istio-proxy**: Main container that receives configuration via xDS from the istiod controller.

1. **Metrics-exporter** (Deployment): component that collects data for multicluster configuration metrics.

    The component is created by the Deckhouse controller if the [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) module parameter is enabled.

    It consists of the following containers:

    * **metrics-exporter**: Main container.
    * **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC, providing secure access to metrics.

1. **Api-proxy** (Deployment): component that provides remote clusters with read access to this cluster's service mesh configuration and application resources. Used by the remote cluster's istiod for multicluster service discovery.

    The component is created by the Deckhouse controller if the [`.settings.multicluster.enabled`](/modules/istio/configuration.html#parameters-multicluster-enabled) module parameter is enabled.

    It consists of a single container:

    * **api-proxy**: Main container.

Component that is not part of the [`istio`](/modules/istio/) module:

* **User application**: workload created by the user and modified by istiod when the pod is mutated.

   It consists of the following containers:

  * **istio-init**: optional init container that sets up iptables rules to intercept application traffic. Added if the [`.settings.dataPlane.trafficRedirectionSetupMode`](/modules/istio/configuration.html#parameters-dataplane-trafficredirectionsetupmode) module parameter is set to `InitContainer`. When it is set to `CNIPlugin`, this function is performed by the istio-cni-node component instead.
  * **istio-proxy**: sidecar container that lets the user application participate in the Istio mesh network. Added if the [`.settings.ambient.enabled`](/modules/istio/configuration.html#parameters-ambient-enabled) module parameter is set to `false` (disabled by default).
  * **user-app**: the user application's own set of init and sidecar containers.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Authorizes requests for the module components' metrics.
   * Manages the Istio, IstioRevision, IngressIstioController, IstioFederation, IstioMulticluster, and WaypointInstance custom resources.
   * Manages custom resources of the `networking.istio.io`, `security.istio.io`, `telemetry.istio.io`, and `extensions.istio.io` API groups.
   * Creates and manages the `istiod-<VERSION>` and `waypoint-<NAME>` Deployments and the `ingress-gateway-controller-<NAME>` DaemonSet.
   * Reads Pod, Namespace, Node, Service, Secret, ConfigMap, Job, CronJob, Deployment, DaemonSet, ReplicaSet, and StatefulSet resources.

1. **The [`user-authn`](/modules/user-authn/) module**: Authenticates Kiali web interface users.
1. **Trickster**: Queries service mesh traffic metrics for the Kiali web interface.
1. **Remote DKP cluster**:

   * Checks the availability of the mesh connection between clusters.
   * Retrieves the remote cluster's service mesh and application parameters.

The following external components interact with the module:

1. **Kube-apiserver**:

   * Validates custom resources of the `networking.istio.io`, `security.istio.io`, `telemetry.istio.io`, and `extensions.istio.io` API groups.
   * Mutates pods to add init and sidecar containers.

1. **Prometheus-main**: Collects metrics from all module components.
1. **Load balancer**: Balances incoming traffic to ingress-gateway-controller.
1. **Controller nginx**: Forwards the authenticated user request to the Kiali web interface.
1. **Remote DKP cluster**:

   * Requests the cluster's public metadata.
   * Sends mesh traffic via mTLS SNI passthrough.
   * Reads the cluster's service mesh and application parameters.

1. **User application**: Receives configuration via xDS from the istiod controller.
