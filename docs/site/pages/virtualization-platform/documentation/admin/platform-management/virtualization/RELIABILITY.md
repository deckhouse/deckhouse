---
title: "Reliability"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/reliability.html
---

## Migration / Maintenance Mode

Virtual machine migration is a key feature in managing virtualized infrastructure, enabling the transfer of running virtual machines from one physical node to another without shutting them down. This process is critical for various tasks and scenarios:

- Load Balancing — moving virtual machines between nodes helps evenly distribute the load, ensuring efficient use of server resources.
- Node Maintenance Mode — virtual machines can be moved off nodes that need to be taken out of service for scheduled maintenance or upgrades.
- Virtual Machine firmware updates — migration allows updating the firmware of virtual machines without interrupting their operation.

### Running migration of a virtual machine

Below is an example of migrating a selected virtual machine.

Before starting the migration, check the current status of the virtual machine:

```bash
d8 k get vm
# NAME                                   PHASE     NODE           IPADDRESS     AGE
# linux-vm                              Running   virtlab-pt-1   10.66.10.14   79m
```

As seen, the virtual machine is currently running on the `virtlab-pt-1` node.

To migrate the virtual machine from one node to another, taking into account placement requirements, use the [VirtualMachineOperations](../../../../reference/cr.html#virtualmachineoperations) (`vmop`) resource with the `Evict` type:

```yaml
d8 k apply -f - <<EOF
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineOperation
metadata:
  name: migrate-linux-vm-$(date +%s)
spec:
  # имя виртуальной машины
  virtualMachineName: linux-vm
  # операция для миграции
  type: Evict
EOF
```

After creating the `vmop` resource, run the following command:

```bash
d8 k get vm -w
# NAME                                   PHASE       NODE           IPADDRESS     AGE
# linux-vm                              Running     virtlab-pt-1   10.66.10.14   79m
# linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
# linux-vm                              Migrating   virtlab-pt-1   10.66.10.14   79m
# linux-vm                              Running     virtlab-pt-2   10.66.10.14   79m
```

This command shows the status of the virtual machine during migration. It allows you to observe the movement of the virtual machine from one node to another.

#### Maintenance mode

When performing maintenance on nodes that are running virtual machines, there is a risk of disrupting their operation. To avoid this, you can put the node into maintenance mode after migrating all virtual machines to other available nodes.

To put a node into maintenance mode and migrate virtual machines, run the following command:

```bash
d8 k drain <nodename> --ignore-daemonsets --delete-emptydir-data
```

Where `<nodename>` is the name of the node on which maintenance will be performed and which needs to be cleared of all resources, including system resources.

This command performs several tasks:

- Evacuates all pods from the specified node.
- Ignores DaemonSets to avoid stopping critical services.
- Deletes temporary data stored in emptyDir to free up node resources.

If you only need to evict virtual machines from the node, you can use a more precise command with filtering by the label that corresponds to virtual machines. For this, run the following command:

```bash
d8 k uncordon <nodename>
```

![](/images/virtualization-platform/drain.com.png)

### Recovery after failure

ColdStandby provides a mechanism to restore a virtual machine's operation in case of a node failure where it was running.

To make this mechanism work, the following requirements must be met:

- The virtual machine's launch policy (`.spec.runPolicy`) must be set to one of the following values: `AlwaysOnUnlessStoppedManually`, `AlwaysOn`.
- The fencing mechanism should be enabled on the nodes where virtual machines are running [fencing](../../../../reference/cr.html#nodegroup-v1-spec-fencing-mode).

How the ColdStandby mechanism works with an example:

1. The cluster consists of three nodes `master`, `workerA`, and `workerB`. The fencing mechanism is enabled on worker nodes.
1. The virtual machine `linux-vm` is initially running on the `workerA` node.
1. The `workerA` node fails (e.g., power loss, network failure, etc.).
1. The Kubernetes controller checks the availability of nodes and detects that `workerA` is unavailable.
1. The controller removes the unavailable `workerA` node from the cluster.
1. The virtual machine `linux-vm` is automatically started on another available node — `workerB`.
