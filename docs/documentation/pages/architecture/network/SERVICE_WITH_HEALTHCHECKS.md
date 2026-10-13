---
title: Service-with-healthchecks module
permalink: en/architecture/network/service-with-healthchecks.html
search: service-with-healthchecks, health check, per-port health check, load balancer
description: Architecture of the service-with-healthchecks module in Deckhouse Platform.
---

The [`service-with-healthchecks`](/modules/service-with-healthchecks/) module provides an internal load balancer with per-port health checks.

The module works with the [ServiceWithHealthchecks](/modules/service-with-healthchecks/cr.html#servicewithhealthchecks) custom resource, which defines a Service load balancer with an independent TCP, HTTP, or PostgreSQL health check for each port.

Unlike the regular load balancer, in which readiness probes are tied to the container state, ServiceWithHealthchecks custom resource allows you to set up active probes on individual TCP ports. In this way, each of the load balancers dealing with the same pod can operate independently of the others: a pod that fails the probes of one balancer is still published by the others.

Probes are grouped by the port they check, not by the resource. When a probe fails, the pod stops receiving traffic on that port only, and the other ports of the same resource keep working. When several probes check the same port, all of them have to pass for that port to receive traffic.

The module is CNI-agnostic: the controller and agent only manage standard Kubernetes Service and EndpointSlice objects, so traffic is still routed by the cluster's existing kube-proxy or CNI implementation, exactly as for a regular Service.

For more details about module configuration and usage examples, refer to the [corresponding documentation section](/modules/service-with-healthchecks/).

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`service-with-healthchecks`](/modules/service-with-healthchecks/) module and its interactions with other components of DP are shown in the following diagram:

![Service-with-healthchecks module architecture](../../images/architecture/network/c4-l2-service-with-healthchecks.svg)

## Module components

The module consists of the following components:

1. **Controller** (Deployment): Reconciles ServiceWithHealthchecks resources and performs the following actions:

   - Creates and maintains a Service resource with an empty `selector` for each custom `ServiceWithHealthchecks` resource. Load balancing is determined entirely by the EndpointSlice resources created by the agent component for the corresponding Service resources.

   - Removes obsolete EnpointSlice resources after cluster node removal.

   It consists of the following containers:

   - **controller**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to controller metrics.

1. **Agent** (DaemonSet): Runs on every cluster node and performs active TCP, HTTP, or PostgreSQL health checks against the pods scheduled to its own node according to the specification of the corresponding ServiceWithHealthchecks custom resource. Based on the check results, agent creates and updates an EndpointSlice bound to the child Service maintained by the controller. Component also watches Secret resources of the `network.deckhouse.io/postgresql-credentials` type, used to authenticate PostgreSQL health checks.

   It consists of the following containers:

   - **agent**: Main container.
   - **kube-rbac-proxy**: Sidecar container with an authorization proxy based on Kubernetes RBAC that provides secure access to component metrics.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   - Watches and updates the status of ServiceWithHealthchecks custom resources.
   - Watches Pod resources on its own node and Secret resources of the `network.deckhouse.io/postgresql-credentials` type.
   - Manages Service, EndpointSlice, and Lease resources.
   - Authorizes requests for metrics.

1. **User services deployed in DP**: Sends active TCP, HTTP, or PostgreSQL health checks to the ports of pods on its node specified in the ServiceWithHealthchecks resource.

The following external components interact with the module:

1. **Prometheus-main**: Collects metrics from the agent and controller components.
