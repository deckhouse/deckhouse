---
title: "Virtual processor of virtual machines"
permalink: en/admin/configuration/virtualization/vm-classes-cpu.html
description: "Virtual processor types in VirtualMachineClass and automatic instruction set selection through vCPU Discovery."
search: virtual processor, vCPU Discovery, CPU type, instruction set
---

A virtual machine class defines which processor and which instruction set the guest system sees.

## Virtual processor

The [`.spec.cpu`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu) block defines the CPU that the guest OS sees. It also determines which nodes a VM can migrate between.

{% alert level="warning" %}
The [`.spec.cpu`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu) block can't be changed after the resource is created. To use a different CPU, create a new class.
{% endalert %}

The following examples cover each CPU type.

- A set of CPU instructions required for a VM. Set with the `Features` type:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

  To configure the vCPU in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

  1. In the **CPU settings** block, select `Features` in the **Type** field.
  1. In the **Required set of supported instructions** field, select the instructions you need.
  1. Click **Create**.

- A universal CPU for a given set of nodes. Set with the `Discovery` type:

  ```yaml
  spec:
    cpu:
      discovery:
        nodeSelector:
          matchExpressions:
            - key: node-role.kubernetes.io/control-plane
              operator: DoesNotExist
      type: Discovery
  ```

  To do the same in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

  1. In the **CPU settings** block, select `Discovery` in the **Type** field.
  1. Click **Add** in the **Conditions for creating a universal CPU** → **Labels and expressions** block.
  1. Set **Key**, **Operator**, and **Value**. They correspond to the [`.spec.cpu.discovery.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu-discovery-nodeselector) parameter.
  1. Press **Enter** to confirm the key parameters.
  1. Click **Create**.

- A CPU close to the node CPU. Set with the `Host` type. The guest OS gets almost the full instruction set of the node, so performance is higher than with a fixed model.
  A VM of this class migrates only between nodes with similar CPUs. For example, migration between nodes with Intel and AMD CPUs isn't possible, and neither is migration between CPU generations with different instruction sets.

  ```yaml
  spec:
    cpu:
      type: Host
  ```

  To do the same in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

  1. In the **CPU settings** block, select `Host` in the **Type** field.
  1. Click **Create**.

- The node CPU without changes. Set with the `HostPassthrough` type. A VM of this class migrates only to a node whose CPU exactly matches the CPU of the source node.

  ```yaml
  spec:
    cpu:
      type: HostPassthrough
  ```

  To do the same in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

  1. In the **CPU settings** block, select `HostPassthrough` in the **Type** field.
  1. Click **Create**.

- A specific CPU model with a known instruction set. Set with the `Model` type.
  First, check which models the target node supports:

  ```bash
  d8 k get nodes <NODE_NAME> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model.node.virtualization.deckhouse.io")) | .key | split("/")[1]' -r
  ```

  Where `<NODE_NAME>` is the name of a cluster node.

  Example output:

  ```console
  Broadwell-noTSX
  Broadwell-noTSX-IBRS
  Haswell-noTSX
  Haswell-noTSX-IBRS
  IvyBridge
  IvyBridge-IBRS
  Nehalem
  Nehalem-IBRS
  Penryn
  SandyBridge
  SandyBridge-IBRS
  Skylake-Client-noTSX-IBRS
  Westmere
  Westmere-IBRS
  ```

  Then specify the selected model in the class specification:

  ```yaml
  spec:
    cpu:
      model: IvyBridge
      type: Model
  ```

  To do the same in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

  1. In the **CPU settings** block, select `Model` in the **Type** field.
  1. In the **Model** field, select the CPU model.
  1. Click **Create**.

## vCPU Discovery configuration example

The following example shows how to choose the processor types in a cluster with heterogeneous nodes.

![VirtualMachineClass configuration example](/images/virtualization/vmclass-examples.png)

The example below uses a cluster of four nodes. Two nodes labeled `group=blue` are equipped with the "CPU X" processor with three instruction sets, and the other two labeled `group=green` have the newer "CPU Y" processor with four sets.

{% alert level="info" %}
A CPU instruction set is every command the processor can execute, from addition to memory operations. The set determines which programs run and how fast, and it differs between CPU generations.
{% endalert %}

Three classes suit such a cluster:

- `universal`: VMs start on any node and migrate between all four. The module takes the instruction set common to both processors, so compatibility is maximal, while some capabilities of "CPU Y" stay unused.
- `cpuX`: VMs start only on nodes with "CPU X" and migrate between them, using all instructions of that processor.
- `cpuY`: The same for nodes with "CPU Y".

The classes for such a cluster look as follows:

```yaml
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: universal
spec:
  cpu:
    # An empty discovery means that all cluster nodes are taken into account.
    discovery: {}
    type: Discovery
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuX
spec:
  cpu:
    discovery:
      nodeSelector:
        matchExpressions:
          - key: group
            operator: In
            values: ["blue"]
    type: Discovery
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: cpuY
spec:
  cpu:
    discovery:
      nodeSelector:
        matchExpressions:
          - key: group
            operator: In
            values: ["green"]
    type: Discovery
```
