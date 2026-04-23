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

5. **Fencing-agent** (DaemonSet): Deployed to a node group when the [`spec.fencing`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing) parameter of the NodeGroup custom resource is enabled.

   After startup, the agent sets the labels `node-manager.deckhouse.io/fencing-enabled` and `node-manager.deckhouse.io/fencing-mode` (the label value is taken from `spec.fencing.mode` — either `Watchdog` or `Notify`) on the node. Agents from the same NodeGroup form a gossip cluster based on the [`memberlist`](https://github.com/hashicorp/memberlist) library and continuously monitor each other over the SWIM protocol, without depending on control-plane availability.

   Fencing works in two stages so that short-term connectivity issues do not trigger cascading action:

   - first, the agent checks **quorum** in the gossip cluster (most of the NodeGroup nodes are reachable). The `memberlist` Lifeguard mechanism protects against false positives caused by temporary network delays and load — nodes are not marked "dead" after a single missed packet;
   - if quorum is not reached, the agent performs a fallback check of Kubernetes API availability. This distinguishes the "the node itself lost connectivity" case from the "control-plane failed, but the nodes can still process traffic" case.

   As long as quorum is held or the API is reachable, the agent periodically resets the Watchdog timer. If neither is available, the agent stops resetting the timer, and the kernel triggers a kernel panic once the timer expires. The exact behavior depends on `spec.fencing.mode`:

   * `Watchdog`: The `softdog` kernel module is loaded with `soft_margin=<timeout>` and `soft_panic=1`. When fencing is enabled, automatic node reboot after kernel panic is disabled at the OS level — this prevents the node from coming back with an undefined state before the cloud-provider controller removes the underlying virtual machine.
   * `Notify`: The agent runs and monitors the cluster, but the watchdog is not armed, and the node is not rebooted. The mode is intended for debugging and observation.

   The agent also exposes a local gRPC API over the Unix socket `/tmp/fencing-agent.sock` (methods `GetAll()` and `StreamEvents()`): external consumers (for example, CNI agents) can retrieve the node list and subscribe to node join/leave events without talking to the Kubernetes API.

   The agent honors the maintenance annotations `node-manager.deckhouse.io/fencing-disable`, `update.node.deckhouse.io/approved`, and `update.node.deckhouse.io/disruption-approved`, and temporarily disables the watchdog during planned maintenance operations.

   The `softdog` kernel module is used as the watchdog. The default timeout is 60 seconds (configurable via `spec.fencing.watchdog.timeout`).

   Consists of a single container:

   * **fencing-agent**: Performs the checks described above and writes to `/dev/watchdog` to signal the watchdog.

6. **Fencing-controller**: A controller that watches all nodes labeled with `node-manager.deckhouse.io/fencing-enabled`.

   If a node is unavailable for more than 60 seconds, the controller deletes all pods from the node and then deletes the Node object (in `Watchdog` mode). Deletion of the Node object is picked up by the cloud-provider controller (MCM/CAPI), which deletes the underlying virtual machine and, if needed, provisions a new one — the faulty node is recreated. For nodes running in `Notify` mode (label `node-manager.deckhouse.io/fencing-mode=Notify`), the Node object is preserved.

For details on how fencing handles different node types, see the [How fencing handles different node types](/modules/node-manager/faq.html#how-fencing-handles-different-node-types) section in the `node-manager` FAQ.

7. **Standby-holder** (Deployment): A pod used to reserve nodes. When the [`spec.cloudinstances.standby`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-standby) parameter is enabled in the NodeGroup custom resource, standby nodes are created in all configured [zones](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-zones).

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
