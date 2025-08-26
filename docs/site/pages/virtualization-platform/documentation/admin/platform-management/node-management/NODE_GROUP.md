---
title: "Node groups"
permalink: en/virtualization-platform/documentation/admin/platform-management/node-management/node-group.html
lang: en
---

## Cluster node management

You can manage cluster nodes using the `node-manager` module.
`node-manager` provides the following main features:

1. Management of multiple nodes as a linked group (NodeGroup):
   - Specifying metadata to be applied to all nodes in a group.
   - Monitoring a group of nodes as an entire object: with segmentation of nodes in graphs, aggregation of alerts about node unavailability, and notifications when a certain number or percentage of nodes in a group is unavailable.
1. Installation, update, and configuration of the node software (containerd, kubelet, etc.), and connection of a node to a cluster:
   - Installation of an operating system (refer to the [list of supported operating systems](../../../about/requirements.html#supported-os-for-platform-nodes)) regardless of the infrastructure type, whether in a cloud environment or on physical hardware.
   - Basic operating system (OS) configuration: disabling auto-update, installing required packages, configuring logging settings, etc.
   - Configuration of nginx to balance requests from nodes between API servers (kubelet), including setting up automatic updates of the upstream server list.
   - Installing and configuring the containerd and Kubernetes CRI, adding a node to a cluster.
   - Managing node updates and disruptions:
     - Automatic selection of an acceptable minor version of Kubernetes for a group of nodes based on its configuration (`kubernetesVersion`), the default version of the entire cluster, and the current version of the control plane. The node update isn't allowed if it comes ahead of the control plane update.
     - Only one node in a group is updated at a time, and only when all nodes in the group are available.
     - Two node update options are available:
       - Normal, which always occur automatically.
       - Updates requiring certain disruptions, such as the kernel update, containerd version change, major kubelet version change, and so on. When automatic disruptive updates are enabled, the update is preceded by a node drain process. This setting can be disabled.
   - Monitoring of update status and progress.

1. Cluster scaling.
   - The Deckhouse Virtualization Platform (DVP) can include the target number of nodes in a group using [Cluster API Provider Static (CAPS)](#working-with-static-nodes).

1. Managing Linux users on nodes.

## Node types

DVP is intended to run on bare-metal servers, therefore, the following sections cover the management of `Static` nodes.

To learn about other node types and cloud provider options,
refer to the [Deckhouse Kubernetes Platform (DKP) documentation](/products/kubernetes-platform/documentation/v1/modules/node-manager).

## Node group

To manage nodes, DVP uses node groups that are described in the [NodeGroup](../../../../reference/cr/nodegroup.html) resources. Each node group performs specific tasks, for example:

- A group for control-plane components of Kubernetes.
- A group for monitoring components.
- A group for control-plane components of DVP.
- A group of nodes with virtual machines (vm-worker nodes).
- A group of nodes with container applications (worker nodes) and so on.

Grouping nodes and distributing components between node groups depend on cluster tasks.
For DVP cluster configuration examples, refer to [Platform installation](../../install/steps/base-cluster.html).

Nodes in a group share common metadata and parameters.
This lets you automatically configure them according to the group configuration.
Also, DVP keeps track of the number of nodes in a group and updates the installed software.

The following monitoring features are available for such node groups:

- Grouping of node parameters on group graphs.
- Grouping of node unavailability alerts.
- Alerts about unavailability of a certain number or percentage of nodes in a group.

## Deployment, configuration, and update of Kubernetes nodes

### Deployment of Kubernetes nodes

Deckhouse automatically performs the following immutable operations to deploy cluster nodes:

1. Configuration and optimization of the operating system to work with containerd and Kubernetes:
   - Installation of the necessary packages from the repositories of a corresponding distribution.
   - Configuration of the kernel operation parameters, logging parameters, log rotation, and other system parameters.
1. Installation of the required versions of containerd and kubelet.
   Adding a node to the Kubernetes cluster.
1. Configuration of nginx and updating the upstream list to balance requests from a node to the Kubernetes API.

### Keeping nodes up to date

Two types of updates can be applied to keep cluster nodes up to date:

- **Common updates**: These updates are always applied automatically and don't cause the node to stop or reboot.
- **Disruptive updates**: These updates may include a kernel or containerd version update, a major kubelet version change, and so on.
   For this type of updates, you can select manual or automatic mode using the [disruptions](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-disruptions) parameter section.
   In the automatic mode, the update can only be applied after a node is properly suspended (drained).

Only one node in the group is updated at any given time, and this is only possible when all nodes in the group are in the available state.

The `node-manager` module has a set of built-in monitoring metrics.
These metrics let you track update progress and receive notifications about encountered issues
or when it's necessary to manually grant a certain permission to update.

## Working with static nodes

### Limitations

When working with static nodes, there are certain limitations to the `node-manager` module's features:

- **No node provisioning**: Resources such as bare-metal servers, virtual machines, and linked resources are provisioned manually.
   The subsequent configuration of resources, such as connecting a node to the cluster, setting up monitoring and so on, is made either fully or partly automatically.
- **No node autoscaling**: Using the [staticInstances.count](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances-count) parameter in CAPS lets you maintain the specified number of nodes in a group.
   DVP will attempt to maintain the specified number of nodes in the group,
   cleaning up unnecessary nodes and setting up new ones as needed
   by picking them from the [StaticInstance](../../../../reference/cr/staticinstance.html) resources that are in `Pending` state.

### Managing a static node manually

You can use pre-made scripts to configure or clear up a node and connect or disconnect it from a cluster.

To configure a server (VM) and add a node to the cluster, download and run a special bootstrap script.
This script is generated for each static node group (each NodeGroup resource) and located in the `d8-cloud-instance-manager/manual-bootstrap-for-<NODEGROUP-NAME>` secret.
For an example of adding a static node to a cluster, refer to the [corresponding section](adding-node.html#adding-a-static-node-manually).

To disconnect a cluster node and clean up the server (VM), run the `/var/lib/bashible/cleanup_static_node.sh` script,
which comes with each static node.
For an example of disconnecting a cluster node and cleaning up the server, refer to [Removing a node from a cluster](adding-node.html#removing-a-node-from-a-cluster).

### Managing a static node automatically

The automatic static node management is performed via [CAPS](#configuring-a-node-via-caps).

CAPS connects to the server (VM) using the [StaticInstance](../../../../reference/cr/staticinstance.html) and [SSHCredentials](../../../../reference/cr/sshcredentials.html) resources,
configures the node, and adds it to the cluster.

If necessary (for example, if the [StaticInstance](../../../../reference/cr/staticinstance.html) resource associated with the server is deleted
or the [number of group nodes](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances-count) is reduced),
CAPS connects to the cluster node, clears it, and disconnects it from the cluster.

### Managing an existing node automatically

{% alert level="info" %}
This feature is supported in Deckhouse 1.63 or higher.
{% endalert %}

To transfer an existing cluster node under CAPS management,
prepare the [StaticInstance](../../../../reference/cr/staticinstance.html) and [SSHCredentials](../../../../reference/cr/sshcredentials.html) resources for this node,
as you would do for automatic management following the guidelines above.
When preparing the [StaticInstance](../../../../reference/cr/staticinstance.html) resource, annotate it as `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

### Configuring a node via CAPS

CAPS is an implementation of a declarative management provider for static nodes (bare-metal servers or virtual machines)
for the [Kubernetes Cluster API](https://cluster-api.sigs.k8s.io/).
CAPS is an additional layer of abstraction above the existing Deckhouse functionality
that provides automatic static node configuration and cleanup using scripts generated for each node group.

CAPS provides the following features:

- Configuring a bare-metal server or virtual machine to connect to a Kubernetes cluster.
- Adding a node to a Kubernetes cluster.
- Removing a node from a Kubernetes cluster.
- Cleaning a bare-metal server or virtual machine after a node is disconnected from a Kubernetes cluster.

CAPS uses the following custom resources:

- **[StaticInstance](../../../../reference/cr/staticinstance.html)**: Each StaticInstance resource details a specific host (server or VM) that is managed using CAPS.
- **[SSHCredentials](../../../../reference/cr/sshcredentials.html)**: Contains the SSH credentials required to connect to the host.
  SSHCredentials is specified in the [`credentialsRef`](../../../../reference/cr/staticinstance.html#staticinstance-v1alpha1-spec-credentialsref) parameter of the StaticInstance resource.
- **[NodeGroup](../../../../reference/cr/nodegroup.html)**: The [`staticInstances`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances) parameter section defines the required number of nodes in a group and the filter for the StaticInstance resources that can be used in the group.

CAPS is enabled automatically if you fill in the [`staticInstances`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances) parameter section in the NodeGroup.
If the `staticInstances` parameter section in the NodeGroup is empty,
then you have to manually configure and clean up nodes in that group, instead of using CAPS.
For details on adding and cleaning up a node in a cluster, refer to [Managing a static node manually](#managing-a-static-node-manually).

The general procedure for working with static nodes when using CAPS is as follows:

1. **Preparing resources.**

   Before bringing a bare-metal server or virtual machine under the CAPS management, the following preliminary steps may be necessary:

   - Preparing the storage system, adding mount points, and so on.
   - Installing OS-specific packages.
   - Configuring the network connectivity. For example, between the server and cluster nodes.
   - Configuring the SSH access to the server, creating a user with the root-level access via `sudo`. A good practice is to create a separate user and unique keys for each server.

1. **Creating a [SSHCredentials](../../../../reference/cr/sshcredentials.html) resource.**

   The SSHCredentials resource contains parameters required by CAPS to connect to a server via SSH.
   A single SSHCredentials resource can be used to connect to multiple servers,
   but it's a good practice to create unique users and access keys to connect to each server.
   In this case, the SSHCredentials resource will also be unique for each server.

1. **Creating a [StaticInstance](../../../../reference/cr/staticinstance.html) resource.**

   For every server (VM) in the cluster, an individual StaticInstance resource is created.
   It contains the IP address for connecting and a link to the SSHCredentials resource with the data required for connecting.

   The following is a list of possible states of StaticInstance, the associated servers (VMs), and cluster nodes:

   - `Pending`: The server isn't configured and there's no associated node in the cluster.
   - `Bootstrapping`: The procedure for configuring the server (VM) and connecting the node to the cluster is in progress.
   - `Running`: The server is configured, and the associated node is added to the cluster.
   - `Cleaning`: The procedure of cleaning up the server and disconnecting a node from the cluster is in progress.

   > You can transfer the existing manually-bootstrapped cluster node under CAPS management by annotating its StaticInstance with `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

1. **Creating a [NodeGroup](../../../../reference/cr/nodegroup.html) resource.**

   When using CAPS,
   focus on the [`nodeType`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-nodetype) parameter (it must be `Static`)
   and the [`staticInstances`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances) parameter section
   of the NodeGroup resource.

   The [`staticInstances.labelSelector`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances-labelselector) parameter section defines a filter that CAPS applies to select the StaticInstance resources to be used for a group.
   The filter allows only certain StaticInstance to be used for specific node groups
   and also allows a single StaticInstance to be used in different node groups.
   You can choose not to define a filter to use any available StaticInstance for a node group.

   The [`staticInstances.count`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances-count) parameter specifies the desired number of nodes in the group.
   If the parameter is changed, CAPS starts adding or removing the desired number of nodes (this process runs in parallel).

   Using the data in the [`staticInstances`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances) parameter section,
   CAPS attempts to maintain the specified number of nodes in the group (the [`count`](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances-count) parameter).
   If a node needs to be added to the group,
   CAPS selects the StaticInstance resource that matches the [filter](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-staticinstances-labelselector) and is in the `Pending` state,
   configures the server (VM), and adds the node to the cluster.
   If a node needs to be removed from the group,
   CAPS selects the StaticInstance resource that is in the `Running` state, cleans up the server (VM),
   and disconnects the node from the cluster.
   The corresponding StaticInstance resource then goes to the `Pending` state and can be reused.

   For an example of adding a node, refer to [Adding a static node using CAPS](adding-node.html#adding-a-static-node-using-caps).

## How to interpret node group states?

**Ready**: A node group contains the minimum required number of scheduled nodes with the the `Ready` status for all zones.

Example 1. A node group in the `Ready` state:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 1
status:
  conditions:
  - status: "True"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

Example 2. A node group in the `Not Ready` state:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
  conditions:
  - status: "False"
    type: Ready
---
apiVersion: v1
kind: Node
metadata:
  name: node1
  labels:
    node.deckhouse.io/group: ng1
status:
  conditions:
  - status: "True"
    type: Ready
```

**Updating**: A node group contains at least one node with an annotation prefixed with `update.node.deckhouse.io`
(for example, `update.node.deckhouse.io/waiting-for-approval`).

**WaitingForDisruptiveApproval**: A node group contains at least one node,
which has the annotation `update.node.deckhouse.io/disruption-required`
but doesn't have the annotation `update.node.deckhouse.io/disruption-approved`.

**Scaling**: Calculated only for node groups of the `CloudEphemeral` type.
Can be in `True` state in two cases:

1. When the number of nodes is less than the desired number of nodes in the group.
   That is, when it's necessary to increase the number of nodes in the group.
1. When a node is marked for deletion or when the number of nodes is greater than the desired number of nodes.
   That is, when it's necessary to reduce the number of nodes in the group.

The desired number of nodes is the sum of all replicas in a node group.

Example. The desired number of nodes is 2:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: ng1
spec:
  nodeType: CloudEphemeral
  cloudInstances:
    maxPerZone: 5
    minPerZone: 2
status:
...
  desired: 2
...
```

**Error**: Contains the latest error that occurred when creating a node in a node group.

## Impact of NodeGroup parameters

| NodeGroup parameter                   | Disruption update          | Node provisioning | kubelet restart |
|---------------------------------------|----------------------------|-------------------|-----------------|
| chaos                                 | -                          | -                 | -               |
| cloudInstances.classReference         | -                          | +                 | -               |
| cloudInstances.maxSurgePerZone        | -                          | -                 | -               |
| cri.containerd.maxConcurrentDownloads | -                          | -                 | +               |
| cri.type                              | - (NotManaged) / + (other) | -                 | -               |
| disruptions                           | -                          | -                 | -               |
| kubelet.maxPods                       | -                          | -                 | +               |
| kubelet.rootDir                       | -                          | -                 | +               |
| kubernetesVersion                     | -                          | -                 | +               |
| nodeTemplate                          | -                          | -                 | -               |
| static                                | -                          | -                 | +               |
| update.maxConcurrent                  | -                          | -                 | -               |

For details about all parameters, refer to the [NodeGroup custom resource](../../../../reference/cr/nodegroup.html) description.

Changing the `InstanceClass` or `instancePrefix` parameter in the Deckhouse configuration won't result in a `RollingUpdate`.
Deckhouse will create new `MachineDeployment` objects and delete the old ones.
The number of `MachineDeployment` objects ordered at the same time is determined by the `cloudInstances.maxSurgePerZone` parameter.

During the disruption update, pods are evicted from the node.
If a pod could not be evicted, the eviction attempt is retried every 20 seconds until a global timeout of 5 minutes is reached.
After that, the pods that could not be evicted are removed.

## How do I allocate nodes to specific loads?

{% alert level="warning" %}
You can't use the `deckhouse.io` domain in `labels` and `taints` keys of the NodeGroup resource.
`deckhouse.io` is reserved for Deckhouse components.
Use the `dedicated` or `dedicated.client.com` keys.
{% endalert %}

There are two ways to accomplish this task:

1. You can set labels to `spec.nodeTemplate.labels` in NodeGroup to use them in the [`spec.nodeSelector`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/) or [`spec.affinity.nodeAffinity`](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#node-affinity) parameters.
   In this case, you select nodes that the scheduler will use for running the target application.
1. You cat set taints to `spec.nodeTemplate.taints` in NodeGroup and then remove them using the [`spec.tolerations`](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/) parameter.
   In this case, you disallow running applications on these nodes unless those applications are explicitly allowed.

{% alert level="info" %}
Deckhouse tolerates the `dedicated` taints by default,
so we recommend using the `dedicated` key with any `value` for taints on your dedicated nodes.️

To use custom keys for taints (for example, `dedicated.client.com`),
add the key's value to the array [`.spec.settings.modules.placement.customTolerationKeys`](../../../../reference/mc.html#global-parameters-modules-placement-customtolerationkeys) parameter.
This way, Deckhouse can deploy system components (for example, `cni-flannel`) to these dedicated nodes.
{% endalert %}

### System components

Deckhouse components use labels and taints for node allocation.
System components can be allocated to separate nodes using the following NodeGroup:

```yaml
nodeTemplate:
  labels:
    node-role.deckhouse.io/system: ""
  taints:
    - effect: NoExecute
      key: dedicated.deckhouse.io
      value: system
```

## How do I allocate nodes to virtual machines?

For virtual machines to run on nodes in a specific group,
after creating the group, create a VirtualMachineClass resource with `nodeSelector`.

Example for the `vm-worker` group:

```yaml
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: vm-worker
spec:
  nodeType: Static
```

A VirtualMachineClass resource with `nodeSelector` for the `vm-worker` group:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: vm-worker
spec:
  nodeSelector:
    matchExpressions:
    - key: node.deckhouse.io/group
      operator: In
      values:
        - vm-worker
```

An excerpt from a manifest for a virtual machine that will be running on the `vm-worker` group's nodes:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: vm-name
spec:
  virtualMachineClassName: vm-workers
  # More VM fields ...
```
