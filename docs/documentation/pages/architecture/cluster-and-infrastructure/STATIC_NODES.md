---
title: Static node management
permalink: en/architecture/cluster-and-infrastructure/node-management/static-nodes.html
search: static nodes
description: Architecture of the node-manager module for Static nodes.
---

This page describes the architecture of the [`node-manager`](/modules/node-manager/) module for Static nodes.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`node-manager`](/modules/node-manager/) module and its interactions with other Deckhouse Kubernetes Platform (DKP) components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Node-manager architecture for Static nodes](../../../../images/architecture/cluster-and-infrastructure/c4-l2-static-nodes.png)

## Module components

{% alert level="info" %}
Bashible is a key component of the Cluster & Infrastructure subsystem that enables the operation of the `node-manager` module. However, it is not part of the module itself, as it runs at the OS level as a system service. For Bashible details, refer to the [corresponding documentation section](bashible.html).
{% endalert %}

The module managing Static nodes consists of the following components:

1. **Bashible-api-server**: A [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/) deployed on master nodes. It generates bashible scripts from templates stored in custom resources. When kube-apiserver receives a request for resources containing bashible bundles, it forwards the request to bashible-api-server and returns the generated result. For more details about bashible and bashible-api-server, refer to the [corresponding documentation section](bashible.html).

2. **Capi-controller-manager** (Deployment): Core controllers from the [Kubernetes Cluster API](https://github.com/kubernetes-sigs/cluster-api) project. Cluster API extends Kubernetes to manage clusters as custom resources within another Kubernetes cluster. The capi-controller-manager pod consists of the following containers:

   * **control-plane-manager**: Main container.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to controller metrics.

3. **Caps-controller-manager** (Deployment): CAPI Provider Static (CAPS), an implementation of a provider for declarative management of static nodes (bare-metal servers or virtual machines) in the [Kubernetes Cluster API](https://github.com/kubernetes-sigs/cluster-api) project. It operates as an extension to capi-controller-manager.

   CAPS provides an additional abstraction layer over the existing DKP mechanism for automatic configuration and cleanup of static nodes using scripts generated for each node group. The component is not tied to a specific cloud provider. For more details, refer to the [`node-manager` documentation](/modules/node-manager/#working-with-static-nodes).

4. **Early-oom** (DaemonSet): A pod deployed on every node. It reads resource load metrics from `/proc` and terminates pods under high load before [kubelet](../../kubernetes-and-scheduling/kubelet.html) does. Enabled by default, but can be disabled in the [module configuration](/modules/node-manager/configuration.html#parameters-earlyoomenabled) if it causes issues for normal node operation.

   Includes the following containers:

   * **psi-monitor**: Monitors the *PSI (Pressure Stall Information)* metric, which reflects how long processes wait for resources such as CPU, memory, or I/O.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the early-oom metrics.

5. **Fencing-agent** (DaemonSet): Deployed to a node group when the [`spec.fencing`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing) parameter of the NodeGroup custom resource is enabled.

   The operation principles of the component is described in detail in the [`spec.fencing.mode`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing-mode) parameter description of the NodeGroup resource. For details on how the fencing mechanism handles different node types, refer to [FAQ](/modules/node-manager/faq.html#how-the-fencing-mechanism-handles-different-node-types) in the `node-manager` module documentation.

   Consists of a single container:

   * **fencing-agent**: Performs the necessary checks and writes to `/dev/watchdog` to signal the watchdog.

6. **Fencing-controller**: A controller that watches all nodes labeled with `node-manager.deckhouse.io/fencing-enabled`.

   If a node is unavailable for more than 60 seconds, the controller deletes all pods from the node but **does not delete the Node object** for static nodes (`node.deckhouse.io/type=Static`). This preserves the node's cluster registration so it can return to service after being manually restored.

   For details on how the fencing mechanism handles different node types, refer to [FAQ](/modules/node-manager/faq.html#how-the-fencing-mechanism-handles-different-node-types) in the `node-manager` module documentation.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Works with Cluster API custom resources.
   * Manages Node resources.
   * Authorizes metric requests.

2. Node filesystem:

   * `/proc`: Reads PSI metrics for OOM handling.
   * `/dev/watchdog`: Sends signals to reset the Watchdog timer.

The following external components interact with the module:

1. **Kube-apiserver**:

   * Executes mutating and validating webhooks of capi-controller-manager.
   * Forwards requests for bashible resources to bashible-api-server.

2. **Prometheus-main**:

   * Collects metrics from `node-manager` module components.

## Architecture features specific to Static nodes

1. Users create and configure nodes in the following ways:

   * Manually, using bashible scripts preconfigured in DKP.
   * Manually, with the following node handover to CAPS for automated management.
   * Automatically, using CAPS.

2. **Capi-controller-manager** manages the lifecycle of the cluster and its nodes through higher-level custom resources, without directly provisioning infrastructure. It generates infrastructure-specific custom resources, leaving provisioning to the infrastructure provider, which is deployed by the specific cloud provider module (CAPS for static nodes).

3. **Caps-controller-manager**: Component managing static nodes (partially, without provisioning).
4. Static nodes can be used not only on bare metal, but in cloud as well. In that case, such a node is not managed by cloud-controller-manager, even if one of the cloud providers is enabled. Csi-driver is not installed on such nodes.
5. Automatic node scaling is not supported.
