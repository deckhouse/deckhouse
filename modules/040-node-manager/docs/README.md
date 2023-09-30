---
title: "Managing nodes"
description: Deckhouse manages nodes of a Kubernetes cluster as a related group, configures and updates Kubernetes cluster node components, manages cluster scaling in the cloud, and manages Linux users on nodes.
---

## Primary functions

The `node-manager` module is responsible for managing nodes and has the following primary functions:
1. Managing multiple nodes as a related group (**NodeGroup**):
    * The ability to define metadata that are inherited by all nodes in the group.
    * Monitoring of a group as a single entity (grouping nodes on graphs by groups, grouping alerts about node unavailability, alerts about the unavailability of N or N% of nodes in a group).
2. **Chaos monkey** — the systemic termination of nodes. This feature tests the resilience of cluster elements and running applications.
3. Installing, updating, and configuring the node software (containerd, kubelet, etc.), connecting the node to the cluster:
    * Installing operating system (see the list of [supported OS](../../supported_versions.html#linux)) regardless of the infrastructure used (any cloud/any hardware).
    * The operating system's basic setup (disabling auto-update, installing the necessary packages, configuring logging parameters, etc.).
    * Configuring nginx (and the system for automatically updating the lsit of upstreams) to balance node (kubelet) requests over API servers.
    * Installing and configuring CRI containerd and Kubernetes, adding the node to the cluster.
    * Managing node updates and their downtime (disruptions):
        * Automatic determination of a valid minor Kubernetes version for a node group based on its settings (the kubernetesVersion parameter specified for a group), the default version for the whole cluster, and the current control-plane version (no nodes can be updated ahead of the control-plane update).
        * Only one node of a group can be updated at a time, and only if all the nodes in the group are available.
        * There are two types of node updates:
            * regular updates – always performed automatically;
            * disruption-involving updates (such as updating the kernel, switching docker versions, major change of the kubelet versions, etc.) – you can choose manual or automatic mode. If automatic disruptive updates are enabled, the node is drained before the update (this functionality can be disabled).
    * Monitoring the status and progress of the update.
4. Autoscaling a cluster and provisioning virtual machines in the cloud (with a supported cloud provider):
    * Creating, starting, and connecting virtual machines to the cluster are performed automatically.
    * You can set a range for the number of virtual machines for a node group.
5. Managing Linux users on nodes.

Nodes are managed through `node group` management, and each node group performs specific tasks. Below are examples of node groups pooled by their tasks performed:
- a group of master nodes;
- a group of nodes for routing HTTP(s) traffic (front nodes);
- a group of monitoring-related nodes;
- a group of application nodes (the so-called worker nodes), etc.

The nodes belonging to the group have common parameters and are configured automatically according to the group's parameters. Deckhouse scales groups by adding, excluding, and updating their nodes. Both cloud and bare metal (static) nodes can be combined into a single group. It paves the way for the hybrid clusters, in which you can scale nodes on physical servers via cloud nodes.

The module supports platforms for which there is a corresponding cloud provider module is available. If there is no support for some cloud platform, you can use its resources in the form of static nodes.

The following Managed Kubernetes services are also supported (not all service functionality may be available):
- Google Kubernetes Engine(GKE).

## Node types

The following node types are supported:
- `CloudEphemeral` — such nodes are automatically ordered, created, and deleted in the configured cloud provider.
- `CloudPermanent` — they differ in that their configuration is not taken from the custom resource [nodeGroup](cr.html#nodegroup), but from a special resource `<PROVIDER>ClusterConfiguration` (for example, [AWSClusterConfiguration](../030-cloud-provider-aws/cluster_configuration.html) for AWS). Also, an important difference is that to apply node configuration, you need to run `dhctl converge` (by running Deckhouse installer). An example of a CloudPermanent node of a cloud cluster is a cluster master node.
- `CloudStatic` — a static node (created manually) hosted in the cloud integrated with one of the cloud providers. This node has the CSI running, and it is managed by the cloud-controller-manager. The `Node` object automatically gets the information about the cloud zone and region. Also, if a node gets deleted from the cloud, its corresponding Node object will be deleted in a cluster.
- `Static` — a static node hosted on a bare metal or virtual machine. The cloud-controller-manager does not manage the node even if one of the cloud providers is enabled.

## Node grouping and group management

Grouping and managing nodes as a related group mean that all nodes in the group will have the same metadata derived from the [`NodeGroup`](cr.html#nodegroup) custom resource.

The group monitoring is available for nodes combined into a group:
- Grouping node parameters on group-related graphs.
- Automatic grouping of alerts about node unavailability.
- Alerts about unavailability of N or N% of nodes in a group, etc.

## Automatic deploying, configuring and updating Kubernetes nodes

### Supported platforms and Kubernetes versions

Automatic deployment (partially supported for *static/hybrid* nodes/clusters), configuration, and continuing software updates are supported for all cluster types (cloud or bare metal based).

The supported Kubernetes version is specified in parameters right down to the minor version. If the version is not set, the `node-manager` module will use the version specified in the `control plane` parameters.

### Deploying Kubernetes nodes

Deckhouse automatically deploys cluster nodes by performing the following **idempotent** operations:
- Configuring the OS and optimizing it for containerd and Kubernetes:
  - Installing the needed packages from the distribution's repository.
  - Configuring kernel parameters, logging, log rotation, and other system parameters.
- Installing the appropriate versions of containerd and kubelet; adding the node to the Kubernetes cluster.
- Configuring Nginx and updating the list of upstream resources for balancing node requests to the Kubernetes API.

### Keeping nodes up-to-date

The node-manager module keeps cluster nodes up-to-date according to the minor Kubernetes version [specified](configuration.html). The automatic update system supports two types of updates:
- **Regular**. These updates are performed automatically and do not cause node stops or restarts.
- **Disruption-related** (e.g., a kernel update, switching containerd version, a major change of the kubelet version, etc.). For this type of updates, you can choose manual or automatic mode. In automatic mode, the node is first drained, and then the update is performed.

> Only one node of the group is updated at a time, and only if all the nodes in the group are available.

The `node-manager` module has a set of built-in metrics for monitoring the update process, alerting about issues with the update, or when a decision to proceed needs to be made by an administrator.

## Provisioning nodes on supported cloud platforms

Each supported cloud provider can provision nodes in an automated manner. For this, you have to specify the required parameters for each node or a group of nodes.

Depending on the provider, these parameters can include:
- type of a node, the number of processor cores, the amount of RAM;
- disk size;
- security settings;
- connected networks;
- other parameters.

Creating, starting, and connecting virtual machines to the cluster are performed automatically.

### Autoscaling nodes

There are two ways for setting the number of nodes in a group when nodes are provisioned as part of a group:
- The fixed number of nodes. In this case, Deckhouse will maintain the specified number of nodes (e.g., by provisioning new nodes if the old ones fail).
- The [minimum](cr.html#nodegroup-v1-spec-cloudinstances-minperzone)/[maximum](cr.html#nodegroup-v1-spec-cloudinstances-maxperzone) number of nodes (range). The autoscaling of nodes is triggered when cluster resources are low and the Pods are in the `Pending` state. If you create several node groups with different parameters and [priority](cr.html#nodegroup-v1-spec-cloudinstances-priority), then the group priority will be considered when automatically scaling (first of all, the group with a high priority will be scaled).

## Chaos Monkey

The instrument (you can enable it for each `NodeGroup` individually) for unexpected and random termination of nodes in a systemic manner. Chaos Monkey tests the resilience of cluster elements, applications, and infrastructure components.
