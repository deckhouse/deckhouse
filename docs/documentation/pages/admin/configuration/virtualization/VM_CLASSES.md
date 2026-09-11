---
title: "Virtual machine classes"
permalink: en/admin/configuration/virtualization/vm-classes.html
description: "The VirtualMachineClass resource: the default class, the set of settings, and how they apply to project virtual machines."
search: VirtualMachineClass, virtual machine class, default class
---

A virtual machine (VM) class defines what the project owner doesn't configure: the virtual CPU model, the allowed combinations of cores and memory, and the nodes where a VM can run. These rules are described by the [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) resource, which you use to control how project workloads are distributed across cluster nodes.

On the initial installation, the module creates the `generic` class with the Nehalem CPU model. This model is old but supported by any modern CPU, so VMs of this class start on any cluster node and migrate between nodes without restrictions.

{% alert level="info" %}
The `generic` class matches a CPU with the smallest instruction set, so it isn't suitable for production workloads.

Once all nodes are added to the cluster and configured, create at least one class with the `Discovery` CPU type. DP selects an instruction set available on all nodes at once, so virtual machines can make fuller use of the CPUs while still being able to migrate between nodes. The instruction set is fixed when the resource is created and doesn't change as nodes are added or removed.

For an example of such a class, see [vCPU Discovery configuration example](vm-classes-cpu.html#vcpu-discovery-configuration-example).
{% endalert %}

Classes exist at the cluster level. To list them, run the following command:

```bash
d8 k get virtualmachineclass
```

Example output:

```console
NAME      PHASE   ISDEFAULT   AGE
generic   Ready               6d1h
```

In any class, you can change everything except the [`.spec.cpu`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-cpu) block, because the CPU model is fixed when the resource is created. You can both modify and delete the `generic` class, but it won't be created again, because the module adds it only on the initial installation.

The project owner specifies the class in the [`.spec.virtualMachineClassName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineclassname) parameter of a virtual machine:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # Name of the VirtualMachineClass resource.
  # ...
```

## Default VirtualMachineClass

You can designate one of the classes as the default. DP inserts its name into the [`.spec.virtualMachineClassName`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-virtualmachineclassname) parameter if the project owner doesn't specify a class.

The default class is marked with the `virtualmachineclass.virtualization.deckhouse.io/is-default-class` annotation set to `true`. A cluster can have only one such class, so to designate a new one, first remove the annotation from the current one.

Don't add the annotation to the `generic` class, because a module update can remove it. Create your own class and designate it as the default instead.

1. Check which classes exist in the cluster:

   ```bash
   d8 k get vmclass
   ```

   Example output with no default class:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                      PHASE   ISDEFAULT   AGE
   generic                   Ready               1d
   host-passthrough-custom   Ready               1d
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

1. Designate the default class:

   ```bash
   d8 k annotate vmclass host-passthrough-custom virtualmachineclass.virtualization.deckhouse.io/is-default-class=true
   ```

1. Verify that the annotation is set:

   ```bash
   d8 k get vmclass
   ```

   Example output:

   <!-- markdownlint-disable MD031 -->
   ```console
   NAME                      PHASE   ISDEFAULT   AGE
   generic                   Ready               1d
   host-passthrough-custom   Ready   true        1d
   ```
   {: .nowrap-default }
   <!-- markdownlint-enable MD031 -->

From now on, virtual machines created without a class get the `host-passthrough-custom` class.

## VirtualMachineClass settings

A class consists of three blocks, each responsible for its own group of settings:

{% tabs vmclass-create %}

{% tab "Using the CLI" %}

Describe the class in a VirtualMachineClass resource:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: <VMCLASS_NAME>
  # The annotation designates the class as the default one. It's optional.
  # annotations:
  #   virtualmachineclass.virtualization.deckhouse.io/is-default-class: "true"
spec:
  # Virtual CPU parameters. The block is required and can't be changed after the resource is created.
  cpu: ...

  # Rules for placing virtual machines on nodes. The block is optional.
  # Changes apply to all VMs of this class.
  nodeSelector: ...

  # Resource allocation policy for virtual machines. The block is optional.
  # Changes apply to all VMs of this class.
  sizingPolicies: ...
```

Where `<VMCLASS_NAME>` is the name of the class you're creating.

{% endtab %}

{% tab "Using the web interface" %}

1. Go to the **System** tab, then to **Virtualization** → **VM classes**.
1. Click **Create**.
1. In the form that opens, enter the VM class name in the **Name** field.

{% endtab %}

{% endtabs %}

The blocks are described separately in the following sections.
