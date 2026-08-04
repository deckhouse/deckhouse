---
title: CloudEphemeral node management
permalink: en/architecture/cluster-and-infrastructure/node-management/cloud-ephemeral-nodes.html
search: cloudephemeral nodes
description: Architecture of the node-manager module for CloudEphemeral nodes.
---

This page describes the architecture of the [`node-manager`](/modules/node-manager/) module for CloudEphemeral nodes.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`node-manager`](/modules/node-manager/) module and its interactions with other Deckhouse Kubernetes Platform (DKP) components are shown in the following diagram:

![Node-manager architecture for CloudEphemeral nodes](../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-ephemeral-nodes.png)

## Module components

{% alert level="info" %}
Bashible is a key component of the Cluster & Infrastructure subsystem that enables the operation of the `node-manager` module. However, it is not part of the module itself, as it runs at the OS level as a system service. For Bashible details, refer to the [corresponding documentation section](bashible.html).
{% endalert %}

The module managing CloudEphemeral nodes consists of the following components:

1. **Bashible-api-server**: A [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/) deployed on master nodes. It generates bashible scripts from templates stored in custom resources. When kube-apiserver receives a request for resources containing bashible bundles, it forwards the request to bashible-api-server and returns the generated result. For more details about bashible and bashible-api-server, refer to the [corresponding documentation section](bashible.html).

1. **Node-controller** (Deployment): A controller that manages [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resources lifecycle. Node-controller performs the following operations:

   * Manages [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resources lifecycle.
   * Implements [NodeGroup](/modules/node-manager/cr.html#nodegroup) custom resources validating webhooks using the [Validating Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) mechanism.
   * Implements [NodeGroup](/modules/node-manager/cr.html#nodegroup) and [Instance](/modules/node-manager/cr.html#instance) custom resources conversion webhooks.
   * Cleans up Node resource labels and taints that remain after the [bashible](bashible.html) first run to initialize the node.
   * Ensures [draining a node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/).
   * Applies labels, taints and annotations from the [`spec.nodeTemplate`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodetemplate) section of a NodeGroup custom resource to all Node resources belonging to it.
   * Calculates and updates a NodeGroup custom resource `status` subresource based on aggregated information obtained from the corresponding Node resources and infrastructure custom resources.
   * Sets the `spec.providerId = "static://"` attribute for Static type Node resources if it is missing.
   * Manages the node update lifecycle: approving updates, handling node disruptions, [draining a node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/) and cleanup after a successful update.

   The component includes:

   * **node-controller**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to controller metrics.

1. **Node-group-exporter** (Deployment): A component that exports NodeGroup resource metrics in Prometheus format, containing information about the number of nodes in each node group: the total number, the number of nodes in the `Ready` status, the number of nodes in error, the minimum and maximum number of nodes in the group, etc.

   The component includes:

   * **node-group-exporter**: Main container.
   * **kube-rbac-proxy** Sidecar container providing an RBAC-based authorization proxy for secure access to exporter metrics.

1. **Capi-controller-manager** (Deployment): Core controllers from the [Kubernetes Cluster API](https://github.com/kubernetes-sigs/cluster-api) project. Cluster API extends Kubernetes to manage clusters as custom resources within another Kubernetes cluster.

   The component includes:

   * **control-plane-manager**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to controller metrics.

1. **Cluster-autoscaler** (Deployment): An additional [Kubernetes component](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) that automatically adjusts the number of nodes in the cluster based on workload. For more details, refer to the [node management documentation section](overview.html#cloud-node-scaling).

   The component includes:

   * **cluster-autoscaler**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the cluster-autoscaler metrics.

1. **Fencing-agent** (DaemonSet) and **fencing-controller**: Components that implement the fencing mechanism. The operation principles of both components are described in detail in the [`spec.fencing.mode`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing-mode) parameter description of the NodeGroup resource. For details on how the fencing mechanism handles different node types, refer to [FAQ](/modules/node-manager/faq.html#how-the-fencing-mechanism-handles-different-node-types) in the `node-manager` module documentation.

1. **Standby-holder** (Deployment): A pod used to reserve nodes. When the [`spec.cloudinstances.standby`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) parameter is enabled in the NodeGroup custom resource, standby nodes are created in all configured [zones](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones).

   A standby node is a cluster node with pre-reserved resources available for immediate scaling. This allows cluster-autoscaler to schedule workloads without waiting for node initialization, which may take several minutes.

   The standby-holder pod does not perform useful work. It simply reserves resources to prevent cluster-autoscaler from deleting temporarily unused nodes.

   The pod has the lowest PriorityClass and is evicted when real workloads are scheduled. For details on pod priority and preemption, refer to the [Kubernetes documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).

   The component includes a single container **reserve-resources**.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Retrieves the `kube-system/d8-node-manager-cloud-provider` Secret for cloud connectivity.
   * Works with Cluster API custom resources.
   * Manages Node and NodeGroup resources.
   * Monitors node load.
   * Performs node autoscaling.
   * Authorizes metric requests.

1. Node filesystem:

   * `/dev/watchdog`: Sends signals to reset the Watchdog timer.

{% alert level="info" %}
The module interacts with the `cloud-provider` module via kube-apiserver using the `kube-system/d8-node-manager-cloud-provider` Secret to obtain cloud connection settings and create CloudEphemeral nodes. The `cloud-provider` module also provides provider-specific Cluster API custom resource templates to `node-manager`.
{% endalert %}

The following external components interact with the module:

1. **Kube-apiserver**:

   * Executes validating and conversion webhooks of node-controller.
   * Executes mutating and validating webhooks of capi-controller-manager.
   * Forwards requests for bashible resources to bashible-api-server.

1. **Prometheus-main**:

   * Collects metrics from `node-manager` module components.

## Architecture features specific to CloudEphemeral nodes

1. Nodes are ephemeral and automatically created and deleted by the module.
1. A configured cloud provider module (`cloud-provider-*`) is required for interaction with cloud infrastructure. It also includes csi-driver and cloud-controller-manager.
1. **Capi-controller-manager** manages the lifecycle of the cluster and its nodes through higher-level custom resources, without directly provisioning infrastructure. It generates infrastructure-specific custom resources, leaving provisioning to the `cloud-provider` module.
1. **Cluster-autoscaler** enables node autoscaling.
1. Node reservation is supported.
