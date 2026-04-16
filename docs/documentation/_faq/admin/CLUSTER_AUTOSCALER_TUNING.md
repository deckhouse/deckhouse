---
title: What to do if it takes a long time to switch to custom nodes in lower-priority groups?
subsystems:
  - cluster_infrastructure
lang: en
---

When using multiple node groups with different priorities in a cloud cluster (the [`spec.cloudInstances.priority`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-priority) parameter), switching to nodes from groups with lower priority can take a long time. An example of this scenario is when groups of preemptible nodes (spot, etc.) are set to the highest priority, and if such instances are unavailable, ordering nodes from other groups takes a very long time.

When provisioning nodes, the cluster autoscaler sequentially selects the group with the highest priority that is **not in a backoff state**. A backoff is a temporary lock on a group following a failed attempt to provision a node (for example, due to a lack of instances of the required type in the cloud).

How it works when using multiple node groups:

1. The cluster autoscaler attempts to provision a node in the group with the highest priority.
1. If the node is not provisioned within the time specified by the `max-node-provision-time` parameter, the attempt is considered a failure.
1. The group is marked as `failed` and blocked for the duration specified in the `initial-node-group-backoff-duration` parameter.
1. If the same group fails again, the lockout time is doubled, but does not exceed the time specified in the `max-node-group-backoff-duration` parameter.
1. The cluster autoscaler selects the next highest-priority group that is not locked out.

Here:

| Parameter | Default value | Description |
|----------|----------------------|-----------|
| `initial-node-group-backoff-duration` | 5 minutes | Initial duration of the group lockout after the first failure. Doubles after each failed node ordering attempt|
| `max-node-group-backoff-duration` | 30 minutes | Maximum duration of the group lockout|
| `max-node-provision-time` | 15 minutes | The time after which the cluster autoscaler considers a node provisioning attempt to have failed |

If these parameters are set to their default values, switching to node ordering in lower-priority groups may take a considerable amount of time (for more details, see [this example](#example-of-the-node-ordering-process-with-default-settings)).

These settings can be adjusted to speed up the switching of nodes in lower-priority groups.
To set the desired values for the `initial-node-group-backoff-duration`, `max-node-group-backoff-duration`, and `max-node-provision-time` parameters, edit the `cluster-autoscaler` Deployment object in the `d8-cloud-instance-manager` namespace. The parameters are specified in the `args` field of the `cluster-autoscaler` container. Example:

```yaml
...
     containers:
      - name: cluster-autoscaler
        args:
        - --initial-node-group-backoff-duration=1m # The initial duration of the group lockout after the first failure has been reduced.
        - --max-node-provision-time=5m # The time after which the cluster autoscaler considers a node request to have failed has been reduced.
...
```

#### If you encounter issues with node ordering in a cluster with a single node group

If cloud cluster uses only one node group (for example, exclusively for ordering spot or preemptible nodes) and you encounter issues with ordering such nodes during automatic scaling, follow these steps:

1. Create one or more InstanceClass objects to be used when creating instances in the cluster. In the InstanceClass, specify node types that differ from those listed in the group mentioned above.
   > In DKP, InstanceClass objects vary depending on the provider. Examples: [AWSInstanceClass](/modules/cloud-provider-aws/cr.html#awsinstanceclass), [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass), [YandexInstanceClass](/modules/cloud-provider-yandex/cr.html#yandexinstanceclass).
1. Create one or more new node groups ([NodeGroup](/modules/node-manager/cr.html#nodegroup)). In the [`spec.cloudInstances.classReference`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-classreference) parameter, specify the InstanceClass objects created in the previous step. In the [`spec.cloudInstances.priority`](/modules/node-manager/cr.html#nodegroup-v1-spec-cloudinstances-priority) parameter, set the priority of the node group.
1. If necessary, modify the `max-node-provision-time`, `max-node-group-backoff-duration`, and `initial-node-group-backoff-duration` parameters to speed up switching to lower-priority groups.

#### Example of the node ordering process with default settings

Suppose there are 3 groups of nodes with different priorities:

- **Group A** (priority 50)
- **Group B** (priority 30)
- **Group C** (priority 0)

In this case, the node ordering process might be as follows:

| Time | Event |
|-------|---------|
| 10:00 | The cluster autoscaler attempts to order a node in **A** |
| 10:15 | The attempt to provision a node in **A** within 15 minutes (`max-node-provision-time`) fails → **A** blocked for 5 minutes (`initial-node-group-backoff-duration`) |
| 10:15:xx | The cluster autoscaler attempts to order a node in **B** (next in priority, not blocked)|
| 10:30 | The attempt to provision a node in **B** within 15 minutes (`max-node-provision-time`) fails → **B** blocked for 5 minutes (`initial-node-group-backoff-duration`)|
| 10:30:xx | The cluster autoscaler attempts to order a node in **A** again, as the lockout period (the default value for `initial-node-group-backoff-duration` is 5 minutes) has ended|
| 10:45 | The attempt to order a node in **A** fails again → **A** is blocked for 10 minutes (`initial-node-group-backoff-duration` for **A** is doubled) |
| … | And so on, until the lockout time for **A** and **B** exceeds 15 minutes (`max-node-provision-time`). In this case, the queue may not reach group **C** for about 1.5 hours |
