---
title: Recommendations for configuring cluster nodes and preventing overload
permalink: en/admin/configuration/high-reliability-and-availability/recommendations.html
---

## Node configuration recommendations

### Master nodes

To ensure cluster fault tolerance, **always** use at least three master nodes.

This number ensures uninterrupted cluster operation and lets you update master nodes safely.
More than three nodes are unnecessary,
and two nodes are insufficient to maintain alignment (quorum) in case of issues in one of the nodes.

Using only one master node means its failure will bring down the entire cluster
since it manages key components that keep the cluster running.

### Frontend nodes

Frontend nodes balance incoming traffic.
Ingress controllers run on them.

Use more than one frontend node.
These nodes must be able to handle traffic in case at least one frontend node fails.

For example, if the cluster has two frontend nodes, each must be capable of handling the full load if the other fails.
If there are three, each node must be able to handle at least a 150% load increase.

### Monitoring nodes

Monitoring nodes run Grafana, Prometheus, and other monitoring components.

In heavily loaded clusters with multiple alerts and large metric volumes,
it's recommended that you allocate dedicated nodes for monitoring.
Otherwise, monitoring components will run on system nodes.

When dedicating nodes for monitoring, it's important they use fast disks.

### System nodes

System nodes are used to run Deckhouse modules.

Allocate two system nodes.
This ensures Deckhouse modules will run on them without interfering with user applications in the cluster.

## Inter-cluster cooperation

You can configure increased fault tolerance not only within a single cluster
but also across multiple clusters using the Service Mesh mode of the `istio` module.

This mode enables federation between two or more clusters,
allowing traffic redistribution if one of them has issues.

Learn more about configuring this mode in [the Service Mesh configuration section](../network/cluster-federation.html).

## Chaos engineering

DKP provides chaos engineering tools to test cluster resilience
by predictably or randomly disrupting components and observing how the infrastructure responds.

Read about configuring these tools in the [Chaos engineering section](./chaos-engineering.html).

## Preventing Kubernetes API overload (FlowSchema)

By default, a DKP cluster includes a component that implements [FlowSchema](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#flowschema) and [PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#prioritylevelconfiguration) to prevent Kubernetes API overload.

`FlowSchema` assigns `PriorityLevel` for `list` requests from all service accounts in Deckhouse namespaces
(with the label `heritage: deckhouse`) to the following apiGroups:

- `v1` (Pod, Secret, ConfigMap, Node, etc.):
  Useful for clusters with numerous basic resources (for example, Secrets or Pods).
- `apps/v1` (DaemonSet, Deployment, StatefulSet, ReplicaSet, etc.):
  Useful when numerous applications are deployed in the cluster (for example, Deployments).
- `deckhouse.io` (Deckhouse custom resources): Useful when there are numerous Deckhouse custom resources in the cluster.
- `cilium.io` (Cilium custom resources): Useful for clusters with numerous Cilium policies.

All API requests matching a `FlowSchema` are placed into a single queue.

This component does not have settings, but the following commands are available:

- Check the state of priority levels:

  ```shell
  d8 k get --raw /debug/api_priority_and_fairness/dump_priority_levels
  ```

- Check the state of priority level queues:

  ```shell
  d8 k get --raw /debug/api_priority_and_fairness/dump_queues
  ```

The component also provides the following metrics to Grafana:

- `apiserver_flowcontrol_rejected_requests_total`: Total number of rejected requests.
- `apiserver_flowcontrol_dispatched_requests_total`: Total number of processed requests.
- `apiserver_flowcontrol_current_inqueue_requests`: Number of requests in queues.
- `apiserver_flowcontrol_current_executing_requests`: Number of requests being executed.
