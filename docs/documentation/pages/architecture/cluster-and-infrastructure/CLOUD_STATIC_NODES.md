---
title: CloudStatic node management
permalink: en/architecture/cluster-and-infrastructure/node-management/cloud-static-nodes.html
search: cloudstatic nodes
description: Architecture of the node-manager module for CloudStatic nodes.
---

This page describes the architecture of the [`node-manager`](/modules/node-manager/) module for CloudStatic nodes.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`node-manager`](/modules/node-manager/) module and its interactions with other Deckhouse Kubernetes Platform (DKP) components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Node-manager architecture for CloudStatic nodes](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-static-nodes.png)

## Module components

{% alert level="info" %}
Bashible is a key component of the Cluster & Infrastructure subsystem that enables the operation of the `node-manager` module. However, it is not part of the module itself, as it runs at the OS level as a system service. For Bashible details, refer to the [corresponding documentation section](bashible.html).
{% endalert %}

The module managing CloudStatic nodes consists of the following components:

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

5. **Fencing-agent** (DaemonSet) and **Fencing-controller**: Components that implement node fencing. For a detailed description, see the [Static node management](static-nodes.html#module-components) page. For details on how fencing handles different node types, see the [How fencing handles different node types](/modules/node-manager/faq.html#how-fencing-handles-different-node-types) section in the `node-manager` FAQ.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Works with Cluster API custom resources.
   * Manages Node resources.
   * Authorizes metric requests.

2. Node filesystem:

   * `/proc`: Reads PSI metrics for OOM handling.
   * `/dev/watchdog`: Sends signals to reset the Watchdog timer.

3. Infrastructure:

    * Manages static nodes (partially, without provisioning).

The following external components interact with the module:

1. **Kube-apiserver**:

   * Executes mutating and validating webhooks of capi-controller-manager.
   * Forwards requests for bashible resources to bashible-api-server.

2. **Prometheus-main**:

   * Collects metrics from `node-manager` module components.

## Architecture features specific to CloudStatic nodes

1. Users create and configure nodes in the following ways:

   * Manually, using bashible scripts preconfigured in DKP.
   * Manually, with the following node handover to CAPS for automated management.
   * Automatically, using CAPS.

2. **Capi-controller-manager** manages the lifecycle of the cluster and its nodes through higher-level custom resources, without directly provisioning infrastructure. It generates infrastructure-specific custom resources, leaving provisioning to the infrastructure provider (CAPS).

3. **Caps-controller-manager**: Component managing static nodes (partially, without provisioning).
4. **Csi-driver** is used to provision disks in the cloud infrastructure.
5. **Cloud-controller-manager** is used to provision load balancers and other infrastructure resources according to its specification.
6. **Infrastructure-provider** of a specific cloud is not required, as CAPS acts on its behalf.
7. Automatic node scaling is not supported.
