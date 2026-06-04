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

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Node-manager architecture for CloudEphemeral nodes](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-ephemeral-nodes.png)

## Module components

{% alert level="info" %}
Bashible is a key component of the Cluster & Infrastructure subsystem that enables the operation of the `node-manager` module. However, it is not part of the module itself, as it runs at the OS level as a system service. For Bashible details, refer to the [corresponding documentation section](bashible.html).
{% endalert %}

The module managing CloudEphemeral nodes consists of the following components:

1. **Bashible-api-server**: A [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/) deployed on master nodes. It generates bashible scripts from templates stored in custom resources. When kube-apiserver receives a request for resources containing bashible bundles, it forwards the request to bashible-api-server and returns the generated result. For more details about bashible and bashible-api-server, refer to the [corresponding documentation section](bashible.html).

2. **Capi-controller-manager** (Deployment): Core controllers from the [Kubernetes Cluster API](https://github.com/kubernetes-sigs/cluster-api) project. Cluster API extends Kubernetes to manage clusters as custom resources within another Kubernetes cluster. The capi-controller-manager pod consists of the following containers:

   * **control-plane-manager**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to controller metrics.

3. **Cluster-autoscaler** (Deployment): An additional [Kubernetes component](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler) that automatically adjusts the number of nodes in the cluster based on workload. For more details, refer to the [node management documentation section](overview.html#cloud-node-scaling).

   The component includes:

   * **cluster-autoscaler**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the cluster-autoscaler metrics.

4. **Early-oom** (DaemonSet): A pod deployed on every node. It reads resource load metrics from `/proc` and terminates pods under high load before [kubelet](../../kubernetes-and-scheduling/kubelet.html) does. Enabled by default, but can be disabled in the [module configuration](/modules/node-manager/configuration.html#parameters-earlyoomenabled) if it causes issues for normal node operation.

   Includes the following containers:

   * **psi-monitor**: Monitors the *PSI (Pressure Stall Information)* metric, which reflects how long processes wait for resources such as CPU, memory, or I/O.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the early-oom metrics.

5. **Fencing-agent** (DaemonSet) and **fencing-controller**: Components that implement the fencing mechanism. The operation principles of both components are described in detail in the [`spec.fencing.mode`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing-mode) parameter description of the NodeGroup resource. For details on how the fencing mechanism handles different node types, refer to [FAQ](/modules/node-manager/faq.html#how-the-fencing-mechanism-handles-different-node-types) in the `node-manager` module documentation.

6. **Standby-holder** (Deployment): A pod used to reserve nodes. When the [`spec.cloudinstances.standby`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) parameter is enabled in the NodeGroup custom resource, standby nodes are created in all configured [zones](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones).

   A standby node is a cluster node with pre-reserved resources available for immediate scaling. This allows cluster-autoscaler to schedule workloads without waiting for node initialization, which may take several minutes.

   The standby-holder pod does not perform useful work. It simply reserves resources to prevent cluster-autoscaler from deleting temporarily unused nodes.

   The pod has the lowest PriorityClass and is evicted when real workloads are scheduled. For details on pod priority and preemption, refer to the [Kubernetes documentation](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).

   The pod includes a single container **reserve-resources**.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Retrieves the `kube-system/d8-node-manager-cloud-provider` Secret for cloud connectivity.
   * Works with Cluster API custom resources.
   * Manages Node resources.
   * Monitors node load.
   * Performs node autoscaling.
   * Authorizes metric requests.

2. Node filesystem:

   * `/proc`: Reads PSI metrics for OOM handling.
   * `/dev/watchdog`: Sends signals to reset the Watchdog timer.

{% alert level="info" %}
The module interacts with the `cloud-provider` module via kube-apiserver using the `kube-system/d8-node-manager-cloud-provider` Secret to obtain cloud connection settings and create CloudEphemeral nodes. The `cloud-provider` module also provides provider-specific Cluster API custom resource templates to `node-manager`.
{% endalert %}

The following external components interact with the module:

1. **Kube-apiserver**:

   * Executes mutating and validating webhooks of capi-controller-manager.
   * Forwards requests for bashible resources to bashible-api-server.

2. **Prometheus-main**:

   * Collects metrics from `node-manager` module components.

## Architecture features specific to CloudEphemeral nodes

1. Nodes are ephemeral and automatically created and deleted by the module.
2. A configured cloud provider module (`cloud-provider-*`) is required for interaction with cloud infrastructure. It also includes csi-driver and cloud-controller-manager.
3. **Capi-controller-manager** manages the lifecycle of the cluster and its nodes through higher-level custom resources, without directly provisioning infrastructure. It generates infrastructure-specific custom resources, leaving provisioning to the `cloud-provider` module.
4. **Cluster-autoscaler** enables node autoscaling.
5. Node reservation is supported.
