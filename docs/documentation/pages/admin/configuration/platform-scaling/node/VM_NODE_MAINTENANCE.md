---
title: "Maintenance of nodes running virtual machines"
permalink: en/admin/configuration/platform-scaling/node/vm-node-maintenance.html
description: "Taking a node with virtual machines out for maintenance: live migration, maintenance mode, restarting non-migratable machines, node shutdown and reboot."
search: node maintenance, drain, maintenance mode, VM migration, node eviction
---

Work on a node affects the virtual machines (VMs) running on it. The sections below cover how to move machines to other nodes, how to take a node out for maintenance, and what to do with machines that cannot be moved.

## VM migration and node maintenance

Live migration moves a running virtual machine from one node to another without shutting it down. It's needed in three situations:

- Load balancing, to distribute VMs evenly across nodes.
- Node maintenance or update, to free the node from VMs.
- Virtual machine firmware update, which would otherwise require a restart.

{% alert level="warning" %}
Live migration is limited in speed and in the number of concurrent moves:

- A node prepares and sends the memory of only one VM at a time, and accepts only one incoming migration at a time.
- This also sets the cluster limit: no more concurrent migrations than there are nodes allowed to run virtual machines.
- The transfer rate of a single migration is limited to 640 MiB/s, which is about 5 Gbit/s.
{% endalert %}

### Moving a selected VM to another node

The following steps show how to move a selected VM to another node.

{% tabs vm-migrate %}

{% tab "Using the CLI" %}

1. Check which node the VM currently runs on:

   ```bash
   d8 k get vm
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       PHASE     UPTIME   NODE           IPADDRESS     AGE
   linux-vm   Running   79m      virtlab-pt-1   10.66.10.14   79m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   The VM runs on the `virtlab-pt-1` node.

1. Create a [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource with the `Evict` type. DP selects a new node for the VM, respecting its placement requirements:

   ```bash
   d8 k create -f - <<EOF
   apiVersion: virtualization.deckhouse.io/v1alpha2
   kind: VirtualMachineOperation
   metadata:
     generateName: evict-linux-vm-
   spec:
     # Virtual machine name.
     virtualMachineName: linux-vm
     # Operation for the migration.
     type: Evict
   EOF
   ```

1. Right after creating the resource, follow the migration progress:

   ```bash
   d8 k get vm -w
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME       PHASE       UPTIME   NODE           IPADDRESS     AGE
   linux-vm   Running     79m      virtlab-pt-1   10.66.10.14   79m
   linux-vm   Migrating   79m      virtlab-pt-1   10.66.10.14   79m
   linux-vm   Migrating   79m      virtlab-pt-1   10.66.10.14   79m
   linux-vm   Running     79m      virtlab-pt-2   10.66.10.14   79m
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

   The VM keeps its IP address during the move; only the node in the `NODE` column changes.

1. To interrupt the migration, delete the created resource while it's in the `Pending` or `InProgress` phase.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **Projects** tab and select the project you need.
1. Go to **Virtualization** → **Virtual machines**.
1. Select the virtual machine from the list and click the ellipsis button.
1. In the menu that opens, select **Migrate**.
1. In the **Virtual machine migration** window, select **Migrate to an arbitrary node** or **Migrate to a selected node** and specify the node in the **Nodes available for migration** field.
1. If required, enable **Migrate disks** to move the VM disks along with the VM, and **Force (slow down guest CPU)** to make sure the migration completes for an actively running VM.
1. Click **Migrate**, or cancel the operation with **Cancel**.

{% endtab %}

{% endtabs %}

### Dedicated migration network

By default, live migration traffic goes over the main node network and competes for bandwidth with workloads. You can route it through a dedicated VLAN provided by the [`sdn`](/modules/sdn/) module.

This requires the [`sdn`](/modules/sdn/) module to be enabled and a [SystemNetwork](/modules/sdn/cr.html#systemnetwork) resource to be created and in the `Ready` state.

To route the traffic to the dedicated network, set the [`.spec.settings.liveMigration.network`](/modules/virtualization/configuration.html#parameters-livemigration-network) block in the `virtualization` ModuleConfig. Specify `type: SystemNetwork` in it and the name of the prepared network in the `systemNetwork.name` field. After that, all migrations in the cluster go over the VLAN of that network.

```yaml
spec:
  settings:
    liveMigration:
      network:
        type: SystemNetwork
        systemNetwork:
          name: migration-net
```

To return migration traffic to the main node network, delete the `network` block. When it isn't set, the node network is used by default.

To create a system network in the web interface:

1. Go to the **System** tab, then to **Network** → **SDN** → **System networks**.
1. Click **Create**.
1. In the **Create resource** window that opens, enter the network name in the **Name** field.
1. On the **Configuration** tab, select the type (`VLAN`, `Access`, or `SRIOVVirtualFunction`) in the **Type** field, the underlay network in the **Underlay network** field, and the VLAN identifier in the **VLAN** block. Set the **IPAM** block parameters, if required.
1. Click **Apply**.
1. Review the created networks in the list, which shows the **Status**, **Type**, **Underlay network**, **VLAN ID**, and **IP pool** columns.

Beforehand, create a network class (VLAN ID ranges and parent network interfaces of nodes) and an underlay network (participating network interfaces of nodes and the **Dedicated** or **Shared** mode) in the **Network** → **SDN** → **Network classes** and **Underlay networks** sections.

### Checking VMs before node maintenance

It's better to find the VMs that won't be able to migrate off a node before maintenance starts, before the node is made unschedulable:

```bash
d8 k get vm -o wide | grep <NODE_NAME>
```

VMs with the `False` value in the `MIGRATABLE` column have to be stopped when the node is drained. Live migration isn't possible for them, and the evacuation fails.

The column value alone isn't enough. A VM with the `True` value and the `VirtualMachineWaitingForMigrationTarget` reason won't move anywhere either, until a suitable node returns to scheduling, so before maintenance, look at the reasons for all VMs on the node:

```bash
d8 k get vm -o json | jq -r '.items[] | [.metadata.name, (.status.conditions[] | select(.type=="Migratable") | .reason)] | @tsv'
```

### Maintenance mode

Work on a node that runs virtual machines can disrupt them. To prevent this, switch the node to maintenance mode, and DP moves the VMs to other nodes.

{% tabs node-drain %}

{% tab "Using the CLI" %}

To free the node from all resources, including system ones, run:

```bash
d8 k drain <NODE_NAME> --ignore-daemonsets --delete-emptydir-data
```

To evict only virtual machines from the node, add a label selector:

```bash
d8 k drain <NODE_NAME> --pod-selector vm.kubevirt.internal.virtualization.deckhouse.io/name --delete-emptydir-data
```

Where `<NODE_NAME>` is the name of the node scheduled for maintenance.

After the command runs, the node switches to maintenance mode, and virtual machines can't be started on it.

To return the node to service, stop the `drain` command with `Ctrl+C`, and then run:

```bash
d8 k uncordon <NODE_NAME>
```

![Diagram of virtual machine migration to another node](../../../../images/virtualization/drain.png)

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Nodes**.
1. Select the node from the list, click the ellipsis button, and select **Cordon + Drain** in the menu that opens.
1. To take the node out of maintenance mode, select **Uncordon** in the same menu.

{% endtab %}

{% endtabs %}

### Restarting virtual machines during node maintenance

A virtual machine can't always be moved to another node by live migration. It can be pinned to the node by placement rules or use a device passed through from the node. The `Migratable` condition in the VM status shows the reason. Such a VM keeps running and holds the node, so maintenance can't complete until the VM is restarted.

When DP finds such a VM while switching the node to maintenance mode, it adds the `virtualization.deckhouse.io/virtualmachines-restart-required` annotation to the node. To allow the restart, add the matching annotation to the node:

```bash
d8 k annotate node <NODE_NAME> virtualization.deckhouse.io/virtualmachines-restart-approved=""
```

Where `<NODE_NAME>` is the name of the node being switched to maintenance mode.

Only the VMs that can't be moved by live migration are restarted. The guest OS shuts down gracefully, after which the VM starts according to the [`.spec.runPolicy`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-runpolicy) run policy. For each restart, a [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) resource named `node-maintenance-restart-*` is created. A restart interrupts the applications inside the VM, so coordinate it with the project owners.

The approval doesn't apply to VMs that can be moved by live migration, including those with no suitable node at the moment. Such VMs are moved by live migration as soon as a suitable node appears.

You can grant the approval in advance, while planning the work. Until the node is switched to maintenance mode, the annotation has no effect. DP removes both annotations once the node is free, so one approval covers one maintenance of one node.

DP reacts to VM eviction from a node. If eviction stopped on timeout (the [`.spec.nodeDrainTimeoutSecond`](/modules/node-manager/cr.html#nodegroup-v1-spec-nodedraintimeoutsecond) parameter of the [NodeGroup](/modules/node-manager/cr.html#nodegroup) resource, 10 minutes by default), eviction isn't retried. An approval granted after that doesn't trigger a restart, and you have to free the node manually.

A restart frees the node but doesn't guarantee that the VM starts on another one right away. The limitation that prevents live migration usually prevents the start on another node as well. In that case, the VM stays in the `Pending` phase, and the `Running` condition reports the reason received from the scheduler. Maintenance can continue meanwhile. The VM starts as soon as a suitable node appears, including after the node returns to service with `d8 k uncordon`.

The VM owner sees the same information in the `EvictionRequired` condition of the VM status. While the node is only being prepared for maintenance, the condition is a warning. Once eviction starts, the condition shows what happens to the VM: a move by live migration, a restart by DP, or a wait if the restart isn't approved.

### Shutting down and rebooting a node with virtual machines

Running virtual machines postpone the shutdown and reboot of their node. DP labels their workloads with `pod.deckhouse.io/inhibit-node-shutdown`, and Deckhouse Platform uses this label to delay the node shutdown. The mechanism is available in the DP EE and Ultimate editions, is described in the [`node-manager` module documentation](/modules/node-manager/), and doesn't need to be enabled.

If a shutdown or reboot is requested on a node that still runs virtual machines:

- The node shutdown is postponed for up to three days.
- A message about the workloads holding the shutdown is periodically printed to the node console.

On nodes where the delay mechanism works, the `GracefulShutdownPostpone` condition is always present and always has the `True` status, even when there are no virtual machines on the node and nobody requested a shutdown. What actually happens to the node is shown by the reason in the `reason` field of this condition:

- `WaitingForShutdownSignal`: The mechanism is active and waiting for a node shutdown request.
- `PodsWithLabelAreRunningOnNode`: A node shutdown is requested and postponed, because virtual machines are still running on the node.
- `NoRunningPodsWithLabel`: No virtual machines are left on the node and the shutdown continues; the condition status changes to `False`.

To check the reason, run the following command:

```bash
d8 k get node <NODE_NAME> -o jsonpath='{range .status.conditions[?(@.type=="GracefulShutdownPostpone")]}{.reason}{"\n"}{end}'
```

The shutdown delay doesn't move virtual machines to other nodes, it only keeps the node from shutting down. For this reason, free the node from virtual machines before any work that requires a shutdown or a reboot:

- If the VMs can be migrated, that is, the `Migratable` condition has the `True` status, switch the node to maintenance mode with `d8 k drain`.
- If a VM can't be migrated, that is, the `Migratable` condition has the `False` status because of local disks or devices passed through from the node, stop it with `d8 v stop <VM_NAME>`, and start it with `d8 v start <VM_NAME>` once the work is done.

  Stopping is available only for the `Manual` and `AlwaysOnUnlessStoppedManually` run policies. Check the VM policy:

  ```bash
  d8 k -n <NAMESPACE> get vm <VM_NAME> -o jsonpath='{.spec.runPolicy}'
  ```

  With the `AlwaysOn` policy, the stop command is rejected with the `NotApplicableForVirtualMachineRunPolicy` reason. In that case, change the policy first, and restore the previous value once the work is done:

  ```bash
  d8 k -n <NAMESPACE> patch vm <VM_NAME> --type merge -p '{"spec":{"runPolicy":"AlwaysOnUnlessStoppedManually"}}'
  ```

  Where `<NAMESPACE>` is the project namespace, and `<VM_NAME>` is the virtual machine name.

Instead of stopping VMs manually, you can [let DP restart such VMs](#restarting-virtual-machines-during-node-maintenance) for the duration of the node maintenance. The run policy doesn't have to be changed in that case.

If you do none of this, the node doesn't shut down. Two alerts report this situation. The `D8VirtualizationVirtualMachineHoldsNodeMaintenance` alert lists the VMs that hold the node and wait for an administrator's decision. The `D8VirtualizationNodeEvacuationStuck` alert fires if a VM was evicted from a node but neither migrated nor restarted within 15 minutes.
