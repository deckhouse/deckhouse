---
title: "Node management"
permalink: en/architecture/node.html
---

## Node types and addition mechanics

In Deckhouse, nodes are divided into the following types:

- **Static** — managed manually; `node-manager` does not scale or recreate them.
- **CloudStatic** — created manually or by any external tools, located in the same cloud that is integrated with one of the cloud provider modules:
  - CloudStatic nodes have several features related to integration with the cloud provider. These nodes are managed by the `cloud-controller-manager` component, resulting in:
    - zone and region metadata being automatically added to the Node object;
    - when the virtual machine is deleted in the cloud, the corresponding Node object is also removed from the cluster;
    - CSI driver can be used to attach cloud volumes.
- **CloudPermanent** — persistent nodes created and updated by `node-manager`.
- **CloudEphemeral** — temporary nodes, created and scaled based on demand.

Nodes are added to the cluster by creating a `NodeGroup` object, which describes the type, parameters, and configuration of the node group. For `CloudEphemeral` groups, DKP interprets this object and automatically creates the corresponding nodes, registering them in the Kubernetes cluster. For other types (e.g., `CloudPermanent` or `Static`), node creation and registration must be done manually or via external tools.

Hybrid groups are also supported, where a single NodeGroup can include both cloud and static nodes. For example, the main load may be handled by bare-metal servers, while cloud instances are used as scalable additions during peak loads.

## Automatic deployment, configuration, and update of Kubernetes nodes

Automatic deployment (in *static/hybrid* — partial), configuration, and software updates work in all clusters regardless of being cloud or bare metal.

### Node Deployment

Deckhouse automatically deploys cluster nodes by performing the following **idempotent** operations:

- OS setup and optimization for working with `containerd` and Kubernetes:
  - required packages are installed from distribution repositories;
  - kernel parameters, journaling settings, log rotation, and other system parameters are configured.
- Installation of required versions of `containerd` and `kubelet`, and registration of the node in the Kubernetes cluster.
- Nginx setup and updating the upstream list for balancing requests from the node to the Kubernetes API.

### Maintaining node state

Two types of updates can be applied to maintain the node's up-to-date state:

- **Regular** — always applied automatically and do not cause downtime or reboot.
- **Disruptive** — such as kernel or containerd updates, significant kubelet version changes, etc. These can be configured in manual or automatic mode via the `disruptions` section. In automatic mode, a node drain is performed before the update.

Only one node per group is updated at a time, and only if all nodes in the group are available.

DKP includes built-in monitoring metrics to track update progress, notify of issues, or prompt for approval if needed.

## Working with Nodes in supported clouds

Each supported cloud provider allows automatic node provisioning. You need to specify required parameters for each node or group.

Depending on the provider, parameters may include:

- node type or CPU/memory capacity;
- disk size;
- security settings;
- connected networks, etc.

VM creation, startup, and cluster joining are performed automatically.

### Cloud node scaling

Two node scaling modes are available:

- **Automatic scaling.**

  When resources are insufficient and pods are in `Pending` state, nodes are added. When there is no load, nodes are removed. Group priority is considered (groups with higher priority are scaled first).
  
  To enable auto-scaling, specify different non-zero values for `minPerZone` and `maxPerZone`.

- **Fixed node count.**

  Deckhouse will maintain the specified number of nodes (e.g., replacing failed ones).

  To disable auto-scaling and maintain a fixed count, use the same value for `minPerZone` and `maxPerZone`.

## Working with Static nodes

When working with static nodes, `node-manager` functions are limited as follows:

- **No node provisioning.** Resource allocation (bare-metal servers, VMs, etc.) is manual. Further configuration (joining the cluster, monitoring, etc.) can be fully or partially automated.
- **No auto-scaling.** Maintaining node count is available using Cluster API Provider Static (`staticInstances.count`). Deckhouse tries to match the node count, removing excess and configuring new ones from `StaticInstance` resources in *Pending* state.

Node setup/removal can be performed as:

- **Manually,** using prepared scripts.

  To configure a server (VM) and join it to the cluster, download and run a bootstrap script. It's generated per NodeGroup and stored in the secret `d8-cloud-instance-manager/manual-bootstrap-for-<NODEGROUP-NAME>`.

  To remove a node from the cluster and wipe the server, use `/var/lib/bashible/cleanup_static_node.sh` (already present on the node).

- **Automatically,** using Cluster API Provider Static.

  CAPS connects to the server using `StaticInstance` and `SSHCredentials`, configures and joins the node.

  If needed (e.g., resource removed or count decreased), CAPS cleans and removes the node.

- **Manually, with transition to automatic management** via CAPS.

  > Available since Deckhouse 1.63.

  To transfer an existing cluster node under CAPS control, prepare `StaticInstance` and `SSHCredentials` as usual, and annotate the `StaticInstance` with `static.node.deckhouse.io/skip-bootstrap-phase: ""`.

## Node grouping and management

Grouping and managing nodes as a group means all group nodes will share metadata from the `NodeGroup` custom resource.

Monitoring features for node groups include:

- grouped parameter charts;
- grouped node availability alerts;
- alerts for N unavailable nodes or N% of the group, etc.

## What is the Instance resource

The `Instance` resource in Kubernetes represents an ephemeral virtual machine, without a specific implementation. It's an abstraction used by tools like `MachineControllerManager` or `Cluster API Provider Static`.

The object has no spec. Its status includes:

1. Reference to `InstanceClass` if available.
1. Reference to the Kubernetes Node.
1. Current machine status.
1. Info on how to check machine creation logs (available during creation).

Creating or deleting a machine will create or delete the corresponding `Instance` resource.

You cannot manually create an `Instance`, but you can delete it. This will remove the machine from the cluster (implementation-dependent).

## When node reboot is required

Some node configuration changes may require a reboot.

For example, reboot is needed after modifying `sysctl` parameters like `kernel.yama.ptrace_scope` (modified using `astra-ptrace-lock enable/disable` in Astra Linux).

## NodeGroup Parameter Effects on Update and Restart

| NodeGroup Parameter                  | Disruption update          | Node reprovisioning | Kubelet restart  |
|-------------------------------------|----------------------------|---------------------|------------------|
| chaos                               | -                          | -                   | -                |
| cloudInstances.classReference       | -                          | +                   | -                |
| cloudInstances.maxSurgePerZone      | -                          | -                   | -                |
| cri.containerd.maxConcurrentDownloads | -                        | -                   | +                |
| cri.type                            | - (NotManaged) / + (other) | -                   | -                |
| disruptions                         | -                          | -                   | -                |
| kubelet.maxPods                     | -                          | -                   | +                |
| kubelet.rootDir                     | -                          | -                   | +                |
| kubernetesVersion                   | -                          | -                   | +                |
| nodeTemplate                        | -                          | -                   | -                |
| static                              | -                          | -                   | +                |
| update.maxConcurrent                | -                          | -                   | -                |

For full details, see the [NodeGroup custom resource](cr.html#nodegroup) documentation.

If `InstanceClass` or `instancePrefix` values change in the Deckhouse configuration, no `RollingUpdate` will occur. Instead, new `MachineDeployment` objects will be created and old ones removed. The number of simultaneous deployments is controlled by `cloudInstances.maxSurgePerZone`.

When an update requires a disruption, a drain of the node is initiated. If a pod cannot be evicted, retries occur every 20 seconds for up to 5 minutes. After that, non-evicted pods are forcefully removed.
