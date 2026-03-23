---
title: Switching CNI in the cluster
permalink: en/guides/cni-migration.html
description: Guide on switching (migrating) CNI in a Deckhouse cluster.
lang: en
layout: sidebar-guides
---

This document describes the procedure for changing the network plugin (CNI) in a Deckhouse Kubernetes Platform cluster. The tool used in Deckhouse allows performing the automated migration (e.g., from Flannel to Cilium) with minimal application downtime and without a full restart of the cluster nodes.

{% alert level="danger" %}
* This guide is applicable for DKP version 1.76 and above. For DKP version 1.75 and earlier, use the [Switching CNI from Flannel or Simple bridge to Cilium](/products/kubernetes-platform/documentation/v1/admin/configuration/network/internal/flannel-simple-to-cilium.html) guide.
* The tool is not intended for switching to any (third-party) CNI.
* During the migration process, the target CNI module (`ModuleConfig.spec.enabled: true`) will be automatically enabled, which must be pre-configured by the cluster administrator.
{% endalert %}

{% alert level="warning" %}
* During the migration process, all pods in the cluster using the network (in PodNetwork) created by the current CNI will be restarted. This will cause an interruption in service availability. To minimize the risks of losing critical data, it is highly recommended to stop the operation of the most critical application services yourself before carrying out the work.
* It is recommended to carry out work during an agreed maintenance window.
* Before carrying out work, it is necessary to disable external cluster management systems (CI/CD, GitOps, ArgoCD, etc.) that may conflict with the process (e.g., trying to restore deleted pods prematurely or rolling back settings). Also, ensure that the cluster management system does not enable the old CNI module.
{% endalert %}

Supported CNI switching modes:

|                  | simple-bridge | flannel (hostgw) | flannel (vxlan) | cilium (native) | cilium (vxlan) |
| ---------------- | :-----------: | :--------------: | :-------------: | :-------------: | :------------: |
| simple-bridge    |      🟫       |        🟩        |       🟩        |       🟩        |       🟩       |
| flannel (hostgw) |      🟩       |        🟫        |       🟨        |       🟩        |       🟩       |
| flannel (vxlan)  |      🟩       |        🟨        |       🟫        |       🟩        |       🟩       |
| cilium (native)  |      🟩       |        🟩        |       🟩        |       🟫        |       🟨       |
| cilium (vxlan)   |      🟩       |        🟩        |       🟩        |       🟨        |       🟫       |

There are several methods to switch CNI in a DKP cluster.

## Method 1: Using the d8 network cni-migration command group of the d8 utility (automated switching)

The [d8](/products/kubernetes-platform/documentation/v1/cli/d8/reference/) utility provides a command group for managing the migration process.

### Starting the migration

To start the process, execute the `switch` command, specifying the target CNI (e.g., `cilium`, `flannel`, or `simple-bridge`):

```bash
d8 network cni-migration switch --to-cni cilium
```

This command will create the necessary resource in the cluster and start the migration controller. DKP will automatically deploy the necessary components: Manager and Agents in the `d8-system` namespace.

### Monitoring progress

To monitor the progress in real-time, use the command:

```bash
d8 network cni-migration watch
```

You will see a dynamic interface with the following information:

* **Current phase**: What exactly is happening at the moment (e.g., `CleaningNodes` or `RestartingPods`).
* **Progress**: List of successfully completed stages and current status of pending cluster actions.
* **Errors**: If a problem occurs on any node, it will be displayed in the `Failed Nodes` list.

Main phases of the process:

1. **Preparing**: Validating the request and waiting for the environment to be ready (e.g., webhooks disabled).
2. **WaitingForAgents**: Waiting for migration agents to start on all nodes.
3. **EnablingTargetCNI**: Enabling the target CNI module in the Deckhouse configuration.
4. **DisablingCurrentCNI**: Disabling the current CNI module.
5. **CleaningNodes**: Agents clean up the network settings of the current CNI on the nodes.
6. **WaitingTargetCNI**: Waiting for the new CNI pods (DaemonSet) to be ready.
7. **RestartingPods**: Restarting application pods to switch them to the new network.
8. **Completed**: Migration successfully completed.

### Completion and cleanup

After the migration status changes to `Succeeded`, you must remove the migration resources (controllers and agents) so that they do not consume cluster resources. To do this, use the command:

```bash
d8 network cni-migration cleanup
```

## Method 2: Using the d8 k commands (manual switching)

The user has the option to manage the migration directly via the Kubernetes API.

### Starting the migration

Create a [CNIMigration](/products/kubernetes-platform/documentation/v1/reference/api/cr.html#cnimigration) resource (the example uses the `cni-migration.yaml` manifest) specifying the target CNI:

```yaml
---
apiVersion: network.deckhouse.io/v1alpha1
kind: CNIMigration
metadata:
  name: migration-to-cilium
spec:
  targetCNI: cilium
```

Apply the manifest in the cluster:

```bash
d8 k create -f cni-migration.yaml
```

### Monitoring progress

Monitor the status of the CNIMigration resource:

```bash
d8 k get cnimigration migration-to-cilium -o yaml -w
# OR
watch -n 1 "d8 k get cnimigration migration-to-cilium -o yaml"
```

Pay attention to the fields:

* `status.phase`: Current stage.
* `status.conditions`: Detailed history of transitions.
* `status.failedSummary`: List of nodes with errors.

For detailed diagnostics of a specific node, you can check its local resource:

```bash
d8 k get cninodemigrations
d8 k get cninodemigration NODE_NAME -o yaml
```

To view the logs of migration controllers in the cluster, execute the following commands:

```bash
d8 k -n d8-system get pods -o wide | grep cni-migration
d8 k -n d8-system logs cni-migration-manager-HASH
d8 k -n d8-system logs cni-migration-agent-HASH
```

### Completion and cleanup

After successful completion (the `CNIMigration` status shows condition `Type: Succeeded, Status: True`), delete the resource:

```bash
d8 k delete cnimigration migration-to-cilium
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
d8 k -n d8-system logs cni-migration-agent-HASH
```

Possible reason: inability to delete CNI configuration files due to permissions, stuck processes, or failure to pass Webhooks verification.

### Target CNI pods do not start

If the target CNI (e.g., Cilium) is in `Init:0/1` status, check the logs of its init container `cni-migration-init-checker`. It waits for the node cleanup to complete. If cleanup is not finished (see the point above), the new CNI will not start. In a critical situation, you can edit the Daemonset to remove the `cni-migration-init-checker` init container.

### Migration stuck

If the process has stopped and hasn't moved for a long time:

1. Check `failedSummary` in the `CNIMigration` status.
1. If there are problematic nodes that cannot be fixed (e.g., a node in NotReady status), you can temporarily remove this node from the cluster or try to reboot it.
