---
title: Upmeter module
permalink: en/architecture/observability/upmeter.html
search: upmeter, availability, component health
description: Architecture of the upmeter module in Deckhouse Kubernetes Platform.
---

The [`upmeter`](/modules/upmeter/) module continuously checks platform availability and cluster component health. Probe results are displayed on dashboards.

To learn more about module settings and usage examples, see the [upmeter configuration page](/modules/upmeter/configuration.html).

## Module architecture

{% alert level="info" %}
The following assumptions are used to simplify the diagram:

* The diagram shows direct communication between containers in different Pods.
  In practice, components communicate through Kubernetes Services (internal load balancers). Service names are omitted when obvious from context. In other cases, a service name is shown above the arrow.
* Pods can run with multiple replicas, but only one replica per Pod is shown in the diagram.
{% endalert %}

The level-2 C4 architecture of the [`upmeter`](/modules/upmeter/) module and its interactions with other Deckhouse Kubernetes Platform (DKP) components are shown below.

![Architecture of the upmeter module](../../images/architecture/observability/c4-l2-upmeter.svg)

{% alert level="info" %}
Numbers in the diagram show the user request flow to the `status` and `webui` components:
- Steps 1, 2, and 3 handle requests through Ingress NGINX Controller with mandatory user authentication in the platform-wide auth system provided by the [`user-authn`](/modules/user-authn) module. For details, see the [user-authn architecture section](../iam/user-authn.html).
{% endalert %}

## Module components

The module includes the following components:

1. **Upmeter** (StatefulSet) is a controller that:

   - Watches the custom resource [Downtime](/modules/upmeter/cr.html#downtime) and calculates DKP component availability excluding downtime intervals defined in this resource.
   - Stores DKP component availability metrics in a local SQLite database.
   - Receives and processes DKP component probe data.
   - Handles API requests for platform availability data.
   - Watches the custom resource [UpmeterRemoteWrite](/modules/upmeter/cr.html#upmeterremotewrite) and sends probe results to the endpoint defined in that resource by using the [Prometheus Remote Write](https://prometheus.io/docs/specs/prw/remote_write_spec/) protocol.

   It includes the following containers:

   - **upmeter**: Main container.
   - **migrator**: Init container that applies SQL migrations to the component SQLite database.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC for secure access to the upmeter API.

1. **Upmeter-agent** (DaemonSet) runs on `master` nodes and regularly executes the following probe groups:

   - Control-plane: API server availability and controller health checks.
   - Deckhouse: DKP cluster health and `deckhouse` module controller checks.
   - Extensions: Checks that every extension has at least one `Ready` Pod.
   - Load-balancing: Checks availability of network load balancing services.
   - Monitoring-and-autoscaling: Checks that the Observability subsystem is healthy and gathers metrics from system components.
   - Nginx: Checks that every Ingress Controller has at least one `Ready` Pod.
   - Nodegroups: Checks the number of `desired` nodes in each NodeGroup.
   - Synthetic: Checks network connectivity between cluster nodes with HTTP requests to smoke-mini-[a-e].

   The control-plane group includes the following probes:
   - Apiserver: Upmeter-agent checks Kubernetes API availability.
   - Basic-functionality: Upmeter-agent checks basic Kubernetes API behavior through the ConfigMap lifecycle.
   - Namespace: Upmeter-agent creates the `upmeter-probe-namespace` namespace and removes it after validation.
   - Scheduler: Upmeter-agent creates a Pod named `upmeter-probe-scheduler`, verifies that it is scheduled to any cluster node, and removes it.
   - Controller-manager: Upmeter-agent creates a StatefulSet named `upmeter-probe-controller-manager` with an intentionally missing container in the Pod spec, verifies that the target Pod is created and reaches the expected state, and then removes the StatefulSet.
   - Cert-manager: Upmeter-agent creates a self-signed Certificate named `upmeter-probe-cert-manager`, verifies that cert-manager created the related Secret, and then removes the Certificate and Secret.

   The following flow is used to check the deckhouse controller:
   - Upmeter-agent creates or updates the custom resource `UpmeterHookProbe`.
   - The deckhouse controller watches this resource and runs a hook to update it.
   - Upmeter-agent also watches `UpmeterHookProbe` and validates changes.

   You can disable probes or probe groups by using the [`.spec.settings.disabledProbes` parameter](/modules/upmeter/configuration.html#parameters-disabledprobes).

   Upmeter-agent sends collected probe results to upmeter with an HTTP request: `POST /downtime`.

   It includes the following containers:

   - **agent**: Main container.
   - **chown-volume-data**: Init container that sets required permissions for the `/var/lib/upmeter/agent` directory on a cluster node.
   - **migrator**: Init container that applies SQL migrations to the component SQLite database.

1. **Smoke-mini-[a-e]** (StatefulSet) is used for synthetic connectivity checks. It includes a single **smoke-mini** container. When upmeter-agent sends a request, smoke-mini-[a-e] instances send requests to each other and to the cluster DNS service, then return probe results.

   When the [`upmeter`](/modules/upmeter/) module is installed, the deckhouse controller from the [`deckhouse`](/modules/deckhouse) module registers a hook that distributes StatefulSet instances across different cluster nodes when possible. After that, it rebalances one StatefulSet every minute to another node.

1. **Status** (Deployment) includes a single **status** container and serves a web page with the current availability status of all DKP components.

1. **Webui** (Deployment) includes a single **webui** container and serves a dashboard with per-component availability history.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   - Manages custom resources `UpmeterRemoteWrite` and `UpmeterHookProbe`.
   - Provides custom resources `Downtime`.
   - Authorizes requests to upmeter.
   - Creates, validates, and deletes standard resources: Pod, StatefulSet, Namespace, and Secret, and the custom resource Certificate.

1. **External metric storage systems**: Sends probe results with the Prometheus Remote Write protocol.

The following external components interact with the module:

1. **Prometheus-main**: Uses monitoring rules and metrics related to the upmeter module.

1. **Controller nginx**: Forwards external user requests to the module web interface.
