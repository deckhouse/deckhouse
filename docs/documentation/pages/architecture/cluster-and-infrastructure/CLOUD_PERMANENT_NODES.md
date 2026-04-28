---
title: CloudPermanent node management
permalink: en/architecture/cluster-and-infrastructure/node-management/cloud-permanent-nodes.html
search: cloudpermanent nodes
description: Architecture of the node-manager module for CloudPermanent nodes.
---

This page describes the architecture of the [`node-manager`](/modules/node-manager/) module for CloudPermanent nodes.

## Module architecture

{% alert level="info" %}
The following simplifications are made in the diagram:

* The diagram shows containers in different pods interacting directly with each other. In reality, they communicate via the corresponding Kubernetes Services (internal load balancers). Service names are omitted if they are obvious from the diagram context. Otherwise, the Service name is shown above the arrow.
* Pods may run multiple replicas. However, each pod is shown as a single replica in the diagram.
{% endalert %}

The Level 2 C4 architecture of the [`node-manager`](/modules/node-manager/) module and its interactions with other Deckhouse Kubernetes Platform (DKP) components are shown in the following diagram:

<!--- Source: structurizr code from https://fox.flant.com/team/d8-system-design/doc/-/tree/main/architecture/diagrams/C4_EN --->
![Node-manager architecture for CloudPermanent nodes](../../../../images/architecture/cluster-and-infrastructure/c4-l2-cloud-permanent-nodes.png)

## Module components

{% alert level="info" %}
Bashible is a key component of the Cluster & Infrastructure subsystem that enables the operation of the `node-manager` module. However, it is not part of the module itself, as it runs at the OS level as a system service. For Bashible details, refer to the [corresponding documentation section](bashible.html).
{% endalert %}

The module managing CloudPermanent nodes consists of the following components:

1. **Bashible-api-server**: A [Kubernetes Extension API Server](https://kubernetes.io/docs/tasks/extend-kubernetes/setup-extension-api-server/) deployed on master nodes. It generates bashible scripts from templates stored in custom resources. When kube-apiserver receives a request for resources containing bashible bundles, it forwards the request to bashible-api-server and returns the generated result. For more details about bashible and bashible-api-server, refer to the [corresponding documentation section](bashible.html).

2. **Early-oom** (DaemonSet): A pod deployed on every node. It reads resource load metrics from `/proc` and terminates pods under high load before [kubelet](../../kubernetes-and-scheduling/kubelet.html) does. Enabled by default, but can be disabled in the [module configuration](/modules/node-manager/configuration.html#parameters-earlyoomenabled) if it causes issues for normal node operation.

   Includes the following containers:

   * **psi-monitor**: Monitors the *PSI (Pressure Stall Information)* metric, which reflects how long processes wait for resources such as CPU, memory, or I/O.
   * **kube-rbac-proxy**: Sidecar container providing an RBAC-based authorization proxy for secure access to the early-oom metrics.

3. **Fencing-agent** (DaemonSet) and **fencing-controller**: Components that implement the fencing mechanism. The operation principles of both components are described in detail in the [`spec.fencing.mode`](/modules/node-manager/cr.html#nodegroup-v1-spec-fencing-mode) parameter description of the NodeGroup resource. For details on how the fencing mechanism handles different node types, refer to [FAQ](/modules/node-manager/faq.html#how-the-fencing-mechanism-handles-different-node-types) in the `node-manager` module documentation.

## Module interactions

The module interacts with the following components:

1. **Kube-apiserver**:

   * Manages Node resources.
   * Authorizes metric requests.

2. Node filesystem:

   * `/proc`: Reads PSI metrics for OOM handling.
   * `/dev/watchdog`: Sends signals to reset the Watchdog timer.

The following external components interact with the module:

1. **Kube-apiserver**:

   * Forwards requests for bashible resources to bashible-api-server.

2. **Prometheus-main**:

   * Collects metrics from `node-manager` module components.

## Architecture features specific to CloudPermanent nodes

1. Nodes are persistent and are created, managed, and deleted by the user. Node management is performed not directly in the infrastructure but via the **dhctl** utility executed as part of the DKP installer.
2. `Terraform-manager` is a [module](/modules/terraform-manager/) used for automated management of cloud infrastructure resources. It checks the Terraform state and applies non-destructive changes to infrastructure resources. The module architecture is described on the [corresponding documentation page](../infrastructure/terraform-manager.html).
3. **Csi-driver** is used to provision disks in the cloud infrastructure.
4. **Cloud-controller-manager** is used to provision load balancers and other infrastructure resources according to its specification.
5. **Infrastructure-provider** is not required. All node management operations are performed by the user via the **dhctl** utility and the `terraform-manager` module.
6. Automatic node scaling is not supported.
