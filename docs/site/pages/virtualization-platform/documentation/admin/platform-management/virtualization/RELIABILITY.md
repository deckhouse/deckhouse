---
title: "Reliability"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/reliability.html
---

## Reliability mechanisms

### Migration and maintenance mode

Virtual machine migration is an important feature in virtualized infrastructure management. It allows you to move running virtual machines from one physical node to another without shutting them down. Virtual machine migration is required for a number of tasks and scenarios:

- Load balancing: Moving virtual machines between nodes allows you to evenly distribute the load on servers, ensuring that resources are utilized in the best possible way.
- Node maintenance: Virtual machines can be moved from nodes that need to be taken out of service to perform routine maintenance or software upgrade.
- Upgrading a virtual machine firmware: The migration allows you to upgrade the firmware of virtual machines without interrupting their operation.

{% alert level="warning" %}
Live migration has the following limitations:

- Only one virtual machine can migrate from each node simultaneously.
- The total number of concurrent migrations in the cluster cannot exceed the number of nodes where running virtual machines is permitted.
- The bandwidth for a single migration is limited to 5 Gbps.
{% endalert %}

#### Start migration of an arbitrary machine

The following is an example of migrating a selected virtual machine.

1. Before starting the migration, check the current status of the virtual machine:

   ```bash
   d8 k get vm
   ```

   Example output:

   ```console
   NAME                                   PHASE     NODE           IPADDRESS     AGE
   linux-vm                              Running   virtlab-pt-1   10.66.10.14   79m
   ```

   We can see that it is currently running on the `virtlab-pt-1` node.

1. To migrate a virtual machine from one node to another taking into account the virtual machine placement requirements, the VirtualMachineOperation (`vmop`) resource with the `Evict` type is used. Create this resource following the example:

   ```yaml
   d8 k create -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachineOperation
   metadata:
     generateName: evict-linux-vm-
   spec:
     # Virtual machine name.
     virtualMachineName: linux-vm
     # An operation for the migration.
     type: Evict
   EOF
   ```

1. Immediately after creating the `vmop` resource, run the following command:

   ```bash
   d8 k get vm -w
   ```

   Example output:

   ```console
   NAME                                   PHASE       NODE           IPADDRESS     AGE
   linux-vm                              Running     virtlab-pt-1   10.66.10.14   79m
   linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
   linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
   linux-vm                              Running     virtlab-pt-2   10.66.10.14   79m
   ```

1. If you need to abort the migration, delete the corresponding `vmop` resource while it is in the `Pending` or `InProgress` phase.

How to start VM migration in the web interface:

- Go to the "Projects" tab and select the desired project.
- Go to the "Virtualization" → "Virtual Machines" section.
- Select the desired virtual machine from the list and click the ellipsis button.
- Select `Migrate` from the pop-up menu.
- Confirm or cancel the migration in the pop-up window.

#### Maintenance mode

When working on nodes with virtual machines running, there is a risk of disrupting their performance. To avoid this, you can put a node into the maintenance mode and migrate the virtual machines to other free nodes.

To do this, run the following command:

```bash
d8 k drain <nodename> --ignore-daemonsets --delete-emptydir-data
```

Where `<nodename>` is a node scheduled for maintenance, which needs to be freed from all resources (including system resources).

If you need to evict only virtual machines off the node, run the following command:

```bash
d8 k drain <nodename> --pod-selector vm.kubevirt.internal.virtualization.deckhouse.io/name --delete-emptydir-data
```

After running the `d8 k drain` command, the node will enter maintenance mode and no virtual machines will be able to start on it.

To take it out of maintenance mode, stop the `drain` command (Ctrl+C), then execute:

```bash
d8 k uncordon <nodename>
```

![A diagram showing the migration of virtual machines from one node to another](/../../../../../images/virtualization-platform/drain.png)

How to perform the operation in the web interface:

- Go to the "System" tab, then to the "Nodes" section→ "Nodes of all groups".
- Select the desired node from the list and click the "Cordon + Drain" button.
- To remove it from maintenance mode, click the "Uncordon" button.

### VM Rebalancing

The platform allows you to automatically manage the placement of running virtual machines in the cluster. To enable this feature, activate the `descheduler` module.

Live migration of virtual machines between cluster nodes is used for rebalancing.

After the module is enabled, the system automatically monitors the distribution of virtual machines and maintains optimal node utilization. The main features of the module are:

- Load balancing: The system monitors CPU reservation on each node. If more than 80% of CPU resources are reserved on a node, some virtual machines will be automatically migrated to less-loaded nodes. This helps avoid overloads and ensures stable VM operation.
- Correct placement: The system checks whether the current node meets the mandatory requirements of the virtual machine's requests, as well as rules regarding their relative placement. For example, if rules prohibit placing certain VMs on the same node, the module will automatically move them to a suitable server.

### ColdStandby

ColdStandby provides a mechanism to recover a virtual machine from a failure on a node it was running on.

The following requirements must be met for this mechanism to work:

- The virtual machine startup policy (`.spec.runPolicy`) must be set to one of the following values: `AlwaysOnUnlessStoppedManually`, `AlwaysOn`.
- The [Fencing mechanism](/modules/node-manager/cr.html#nodegroup#nodegroup-v1-spec-fencing-mode) must be enabled on nodes running the virtual machines.

Let's see how it works on the example:

1. A cluster consists of three nodes: `master`, `workerA`, and `workerB`. The worker nodes have the Fencing mechanism enabled. The `linux-vm` virtual machine is running on the `workerA` node.
1. A problem occurs on the `workerA` node (power outage, no network connection, etc.).
1. The controller checks the node availability and finds that `workerA` is unavailable.
1. The controller removes the `workerA` node from the cluster.
1. The `linux-vm` virtual machine is started on another suitable node (`workerB`).
   ![ColdStandBy mechanism diagram](/../../../../../images/virtualization-platform/coldstandby.png)
