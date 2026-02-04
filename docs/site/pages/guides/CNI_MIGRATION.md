---
title: Switching CNI in the cluster
permalink: en/guides/cni-migration.html
description: Guide on switching (migrating) CNI in a Deckhouse cluster.
lang: en
layout: sidebar-guides
---

This document describes the procedure for changing the network plugin (CNI) in a Kubernetes cluster managed by Deckhouse. The tool used in Deckhouse allows performing the automated migration (e.g., from Flannel to Cilium) with minimal application downtime and without a full restart of the cluster nodes.

{% alert level="danger" %}

* The tool is NOT intended for switching to any (third-party) CNI.
* During the migration process, the target CNI module (`ModuleConfig.spec.enabled: true`) will be automatically enabled, which must be pre-configured by the user/administrator.

{% endalert %}

{% alert level="warning" %}

* During the migration process, all pods in the cluster using the network (in PodNetwork) created by the current CNI will be restarted. This will cause an interruption in service availability. To minimize the risks of losing critical data, it is highly recommended to stop the operation of the most critical application services yourself before carrying out the work.
* It is recommended to carry out work during an agreed maintenance window.
* Before carrying out work, it is necessary to disable external cluster management systems (CI/CD, GitOps, ArgoCD, etc.) that may conflict with the process (e.g., trying to restore deleted pods prematurely or rolling back settings). Also, ensure that the cluster management system does not enable the old CNI module.

{% endalert %}

## Method 1: Using the `d8` utility

The `d8` utility provides a convenient interface for managing the migration process.

### 1. Starting the migration

To start the process, execute the `switch` command, specifying the target CNI (e.g., `cilium`, `flannel`, or `simple-bridge`).

```bash
d8 cni-migration switch --to-cni cilium
```

This command will create the necessary resource in the cluster and start the migration controller. Deckhouse will automatically deploy the necessary components: Manager and Agents in the `d8-system` namespace.

### 2. Monitoring progress

To monitor the progress in real-time, use the command (which starts automatically when migration begins):

```bash
d8 cni-migration watch
```

You will see a dynamic interface with the following information:

* **Current phase:** What exactly is happening at the moment (e.g., `CleaningNodes` or `RestartingPods`).
* **Progress:** List of successfully completed stages and current status of pending cluster actions.
* **Errors:** If a problem occurs on any node, it will be displayed in the `Failed Nodes` list.

Main phases of the process:

1. **Preparing:** Validating the request and waiting for the environment to be ready (e.g., webhooks disabled).
2. **WaitingForAgents:** Waiting for migration agents to start on all nodes.
3. **EnablingTargetCNI:** Enabling the target CNI module in the Deckhouse configuration.
4. **DisablingCurrentCNI:** Disabling the current CNI module.
5. **CleaningNodes:** Agents clean up the network settings of the current CNI on the nodes.
6. **WaitingTargetCNI:** Waiting for the new CNI pods (DaemonSet) to be ready.
7. **RestartingPods:** Restarting application pods to switch them to the new network.
8. **Completed:** Migration successfully completed.

### 3. Completion and cleanup

After the migration status changes to `Succeeded`, you must remove the migration resources (controllers and agents) so that they do not consume cluster resources.

```bash
d8 cni-migration cleanup
```

## Method 2: Manual management (via kubectl)

The user has the option to manage the migration directly via the Kubernetes API.

### 1. Starting the migration

Create a `cni-migration.yaml` manifest specifying the target CNI:

```yaml
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-to-cilium
spec:
  targetCNI: cilium
```

Apply it in the cluster:

```bash
kubectl create -f cni-migration.yaml
```

### 2. Monitoring progress

Monitor the status of the `CNIMigration` resource:

```bash
kubectl get cnimigration migration-to-cilium -o yaml -w
# OR
watch -n 1 "kubectl get cnimigration migration-to-cilium -o yaml"
```

Pay attention to the fields:

* `status.phase`: Current stage.
* `status.conditions`: Detailed history of transitions.
* `status.failedSummary`: List of nodes with errors.

For detailed diagnostics of a specific node, you can check its local resource:

```bash
kubectl get cninodemigrations
kubectl get cninodemigration NODE_NAME -o yaml
```

To view the logs of migration controllers in the cluster, execute the following commands:

```bash
kubectl -n d8-system get pods -o wide | grep cni-migration
kubectl -n d8-system logs cni-migration-manager-HASH
kubectl -n d8-system logs cni-migration-agent-HASH
```

### 3. Completion and cleanup

After successful completion (the `CNIMigration` status shows condition `Type: Succeeded, Status: True`), delete the resource:

```bash
kubectl delete cnimigration migration-to-cilium
```

This action signals Deckhouse to remove all previously created resources in the cluster.

## Troubleshooting

{% alert %}

The CNI switching tool does not evaluate the network connectivity of pods and cluster components after the CNI migration in the cluster.

{% endalert %}

### Agent does not start on a node

Check the status of the `cni-migration-agent` DaemonSet in the `d8-system` namespace. There may be taints on the node that are not covered by the agent's tolerations.

### Node stuck in CleaningNodes phase

Check the logs of the agent pod on the corresponding node:

```bash
kubectl -n d8-system logs cni-migration-agent-HASH
```

Possible reason: inability to delete CNI configuration files due to permissions, stuck processes, or failure to pass Webhooks verification.

### Target CNI pods do not start

If the target CNI (e.g., Cilium) is in `Init:0/1` status, check the logs of its init container `cni-migration-init-checker`. It waits for the node cleanup to complete. If cleanup is not finished (see the point above), the new CNI will not start. In a critical situation, you can edit the Daemonset to remove the `cni-migration-init-checker` init container.

### Migration stuck

If the process has stopped and hasn't moved for a long time:

1. Check `failedSummary` in the `CNIMigration` status.
2. If there are problematic nodes that cannot be fixed (e.g., a node in NotReady status), you can temporarily remove this node from the cluster or try to reboot it.
