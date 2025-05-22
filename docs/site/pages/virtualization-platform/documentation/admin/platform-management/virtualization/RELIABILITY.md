---
title: "Reliability"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/reliability.html
---

## Migration / Maintenance Mode

Virtual machine migration is a key feature in managing virtualized infrastructure, enabling the transfer of running virtual machines from one physical node to another without shutting them down. This process is critical for various tasks and scenarios:

- **Load balancing**. Moving virtual machines between nodes allows you to evenly distribute the load on servers, ensuring that resources are utilized in the best possible way.
- **Node maintenance**. Virtual machines can be moved from nodes that need to be taken out of service to perform routine maintenance or software upgrade.
- **Upgrading a virtual machine firmware**. The migration allows you to upgrade the firmware of virtual machines without interrupting their operation.

### Running migration of a virtual machine

Before starting the migration, check the current status of the virtual machine with the following command:

```bash
d8 k get vm
```

In the output, you should see information about the virtual machine:

```console
NAME                                   PHASE     NODE           IPADDRESS     AGE
linux-vm                              Running   virtlab-pt-1   10.66.10.14   79m
```

As seen, the virtual machine is currently running on the `virtlab-pt-1` node.

To migrate the virtual machine from one node to another, taking into account placement requirements, use the [VirtualMachineOperation](../../../../reference/cr/virtualmachineoperation.html) (`vmop`) resource with the `Evict` type:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: migrate-linux-vm-$(date +%s)
spec:
  # Name of virtual machine.
  virtualMachineName: linux-vm
  # Operation.
  type: Evict
EOF
```

After creating the `vmop` resource, run the following command:

```bash
d8 k get vm -w
```

In the output, you should see information about the phase of the virtual machine:

```console
NAME                                   PHASE       NODE           IPADDRESS     AGE
linux-vm                              Running     virtlab-pt-1   10.66.10.14   79m
linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
linux-vm                              Running     virtlab-pt-2   10.66.10.14   79m
```

This command shows the status of the virtual machine during migration. It allows you to observe the movement of the virtual machine from one node to another.

If you need to abort the migration, delete the corresponding `vmop` resource while it is in the `Pending` or `InProgress` phase.

#### Maintenance mode

When performing maintenance on nodes that are running virtual machines, there is a risk of disrupting their operation. To avoid this, you can put the node into maintenance mode after migrating all virtual machines to other available nodes.

To put a node into maintenance mode and migrate virtual machines, run the following command:

```bash
d8 k drain <nodename> --ignore-daemonsets --delete-emptydir-data
```

Where `<nodename>` is the name of the node on which maintenance will be performed and which needs to be cleared of all resources, including system resources.

This command performs several tasks:

- evacuates all pods from the specified node.
- ignores DaemonSets to avoid stopping critical services.
- deletes temporary data stored in `emptyDir` to free up node resources.

If you only need to evict virtual machines from the node, you can use a more precise command with filtering by the label that corresponds to virtual machines. For this, run the following command:

```bash
d8 k drain <nodename> --pod-selector vm.kubevirt.internal.virtualization.deckhouse.io/name --delete-emptydir-data
```

After running the `d8 k drain` command, the node will enter maintenance mode and no virtual machines will be able to start on it.

To take it out of maintenance mode, stop the `drain` command (Ctrl+C), then execute:

```bash
d8 k uncordon <nodename>
```

![Maintenance mode, diagram](/../../../../../images/virtualization-platform/drain.png)

### Recovery after failure (ColdStandby)

ColdStandby provides a mechanism to restore a virtual machine's operation in case of a node failure where it was running.

To make this mechanism work, the following requirements must be met:

- The virtual machine's launch policy (`.spec.runPolicy`) must be set to one of the following values: `AlwaysOnUnlessStoppedManually`, `AlwaysOn`.
- The [Fencing mechanism](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-fencing-mode) mechanism should be enabled on the nodes where virtual machines are running [fencing](../../../../reference/cr/nodegroup.html#nodegroup-v1-spec-fencing-mode).

How the ColdStandby mechanism works with an example:

1. A cluster consists of three nodes: `master`, `workerA`, and `workerB`. The worker nodes have the Fencing mechanism enabled. The `linux-vm` virtual machine is running on the `workerA` node.
1. A problem occurs on the `workerA` node (power outage, no network connection, etc.).
1. The controller checks the node availability and finds that `workerA` is unavailable.
1. The controller removes the `workerA` node from the cluster.
1. The `linux-vm` virtual machine is started on another suitable node (`workerB`).

![Failure recovery, diagram](/../../../../../images/virtualization-platform/coldstandby.png)
