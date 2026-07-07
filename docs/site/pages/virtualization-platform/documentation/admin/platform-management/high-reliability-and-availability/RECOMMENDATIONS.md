---
title: Recommendations for configuring cluster nodes and preventing overload
permalink: en/virtualization-platform/documentation/admin/platform-management/high-reliability-and-availability/recommendations.html
---

Proper node configuration increases cluster fault tolerance and simplifies maintenance.
Below are the recommended roles and number of nodes for various components.

## Master nodes

To ensure cluster fault tolerance, **always** use at least three master nodes.
This number ensures uninterrupted cluster operation and lets you update master nodes safely.
More than three nodes are unnecessary,
and two nodes are insufficient to maintain alignment (quorum) in case of issues in one of the nodes.

Using only one master node means its failure will bring down the entire cluster
since it manages key components that keep the cluster running.

## Frontend nodes

Frontend nodes balance incoming traffic.
Ingress controllers run on them.
Use more than one frontend node.
These nodes must be able to handle traffic in case at least one frontend node fails.

For example, if the cluster has two frontend nodes, each must be capable of handling the full load if the other fails.
If there are three, each node must be able to handle at least a 150% load increase.

## Monitoring nodes

Monitoring nodes run Grafana, Prometheus, and other monitoring components.
In heavily loaded clusters with multiple alerts and large metric volumes,
it's recommended that you allocate dedicated nodes for monitoring.
Otherwise, monitoring components will run on system nodes.

When dedicating nodes for monitoring, it's important they use fast disks (at least 400 IOPS).

## System nodes

System nodes are used to run Deckhouse modules.
Allocate two system nodes.
This ensures Deckhouse modules will run on them without interfering with user applications in the cluster.

## Preventing Kubernetes API overload (FlowSchema)

By default, a DVP cluster includes a component that implements [FlowSchema](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#flowschema) and [PriorityLevelConfiguration](https://kubernetes.io/docs/concepts/cluster-administration/flow-control/#prioritylevelconfiguration) to prevent Kubernetes API overload.

`FlowSchema` assigns `PriorityLevel` for `list` requests from all service accounts in Deckhouse namespaces
(with the label `heritage: deckhouse`) to the following apiGroups:

- `v1` (Pod, Secret, ConfigMap, Node, etc.):
  Useful for clusters with numerous basic resources (for example, Secrets or Pods).
- `apps/v1` (DaemonSet, Deployment, StatefulSet, ReplicaSet, etc.):
  Useful when numerous applications are deployed in the cluster (for example, Deployments).
- `deckhouse.io` (Deckhouse custom resources): Useful when there are numerous Deckhouse custom resources in the cluster.
- `cilium.io` (Cilium custom resources): Useful for clusters with numerous Cilium policies.

All API requests matching a `FlowSchema` are placed into a single queue with a shared priority.
This limits the load on the Kubernetes API and prevents it from being overloaded by mass `list` requests.

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

## Delaying node reboot or shutdown while critical pods are running

{% alert level="info" %}
Available in the **EE** edition.
{% endalert %}

DKP supports a mechanism for delaying the shutdown or reboot of a node if it is running stateful applications or virtual machines (VMs). This is implemented by the `d8-shutdown-inhibitor` service of the [`node-manager`](/modules/node-manager/) module.

The delay is triggered if pods with the label `pod.deckhouse.io/inhibit-node-shutdown` are running on the node. DKP delays the shutdown or restart of the node so that the user can migrate critical pods or terminate them in a controlled manner. This is useful, for example, for stateful workloads and other cases where data loss must be avoided. For information about the mechanism's principles and features, refer to the [`node-manager` module documentation](/modules/node-manager/#delaying-node-reboot-or-shutdown-while-critical-pods-are-running).

### Enabling a node shutdown delay

{% alert level="warning" %}
To decide whether to block a node shutdown, DKP additionally queries the NodeGroup. If the current node belongs to the `master` group and it is the only master node in the cluster, the shutdown block will not be applied to it.
{% endalert %}

To enable the mechanism that delays a node's restart or shutdown, add the label `pod.deckhouse.io/inhibit-node-shutdown` to the Pod (for a Deployment, specify the label in the pod template).

Example of a Pod manifest with the `pod.deckhouse.io/inhibit-node-shutdown` label:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-stateful-app
  labels:
    app: my-stateful-app
    pod.deckhouse.io/inhibit-node-shutdown: "true"
spec:
  containers:
    - name: app
      image: my-registry/my-app:1.0.0
```

{% alert level="warning" %}
The block only applies to pods labeled with `pod.deckhouse.io/inhibit-node-shutdown` that are in the `Running` phase on the node. Kubelet shuts down the pods without this label according to the standard graceful shutdown process.
{% endalert %}

### Searching pods that prevent shutdown and performing the node troubleshooting

To search the pods that are blocking a node shutdown, use the following command:

```shell
d8 k get po -A -l pod.deckhouse.io/inhibit-node-shutdown -o wide
```

To view the `GracefulShutdownPostpone` condition on a node, use the following command:

```shell
d8 k get node <NODE_NAME> -o jsonpath='{range .status.conditions[?(@.type=="GracefulShutdownPostpone")]}{.type}{"\t"}{.status}{"\t"}{.reason}{"\t"}{.message}{"\n"}{end}'
```

Possible values of the `reason` field:

- `WaitingForShutdownSignal`: The shutdown blocking mechanism is running on the node, but the blocking has not yet begun.
- `PodsWithLabelAreRunningOnNode`: The shutdown has been blocked because there are pods with the label `pod.deckhouse.io/inhibit-node-shutdown` still running on the node.
- `NoRunningPodsWithLabel`: The blocking has been released and the shutdown can continue.
