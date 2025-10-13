---
title: "Virtual Machine Classes"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/virtual-machine-classes.html
---

The [`VirtualMachineClass`](/modules/virtualization/cr.html#virtualmachineclass) resource is intended for centralized configuration of preferred virtual machine parameters. It allows setting parameters for CPU, including instructions and resource configuration policies, as well as defining the ratio between CPU and memory resources. Additionally, [VirtualMachineClass](/modules/virtualization/cr.html#virtualmachineclass) manages the placement of virtual machines across the platform nodes, helping administrators efficiently distribute resources and optimally place virtual machines.

During installation, a single VirtualMachineClass `generic` resource is automatically created. It represents a universal CPU type based on the older, but widely supported, Nehalem architecture. This enables running VMs on any nodes in the cluster and allows live migration.

{% alert level="info" %}
It is recommended that you create at least one VirtualMachineClass resource in the cluster with the `Discovery` type immediately after all nodes are configured and added to the cluster. This allows virtual machines to utilize a generic CPU with the highest possible CPU performance considering the CPUs on the cluster nodes. This allows the virtual machines to utilize the maximum CPU capabilities and migrate seamlessly between cluster nodes if necessary.

For a configuration example, see [vCPU Discovery configuration example](#vcpu-discovery-configuration-example)
{% endalert %}

To list all VirtualMachineClass resources, run the following command:

```bash
d8 k get virtualmachineclass
```

Example output:

```console
NAME               PHASE   AGE
generic            Ready   6d1h
```

Make sure to specify the VirtualMachineClass resource in the virtual machine configuration.
The following is an example of specifying a class in the VM specification:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # VirtualMachineClass resource name.
  ...
```

### Default VirtualMachineClass

For convenience, you can assign a default VirtualMachineClass. This class will be used in the `spec.virtualMachineClassName` field if it is not specified in the virtual machine manifest.

The default VirtualMachineClass is set via the `virtualmachineclass.virtualization.deckhouse.io/is-default-class` annotation. There can be only one default class in the cluster. To change the default class, remove the annotation from one class and add it to another.

It is not recommended to set the annotation on the `generic` class, since the annotation may be removed during an update. It is recommended to create your own class and assign it as the default.

To list all VirtualMachineClass resources, run the following command:

```shell
d8 k get virtualmachineclass
```

Example output (no default class):

```console
NAME                                    PHASE   ISDEFAULT   AGE
generic                                 Ready               1d
host-passthrough-custom                 Ready               1d
```

To assign the default class, run:

```shell
d8 k annotate vmclass host-passthrough-custom virtualmachineclass.virtualization.deckhouse.io/is-default-class=true
```

Example output:

```console
virtualmachineclass.virtualization.deckhouse.io/host-passthrough-custom annotated
```

After assigning the default class, list all VirtualMachineClass resources again:

```shell
d8 k get vmclass
```

Example output (with default class):

```console
NAME                                    PHASE   ISDEFAULT   AGE
generic                                 Ready               1d
host-passthrough-custom                 Ready   true        1d
```

When creating a VM without specifying the `spec.virtualMachineClassName` field, it will be set to `host-passthrough-custom`.

## VirtualMachineClass settings

The structure of the `VirtualMachineClass` resource is as follows:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: <vmclass-name>
  # (optional) Set class as a default.
  # annotations:
  #   virtualmachineclass.virtualization.deckhouse.io/is-default-class: "true"
spec:
  # The section describes virtual processor parameters for virtual machines.
  # This block cannot be changed after the resource has been created.
  cpu: ...
  # (optional) Describes the rules for allocating virtual machines between nodes.
  # When changed, it is automatically applied to all virtual machines using this VirtualMachineClass.
  nodeSelector: ...
  # (optional) Describes the sizing policy for configuring virtual machine resources.
  # When changed, it is automatically applied to all virtual machines using this VirtualMachineClass.
  sizingPolicies: ...
```

How to configure VirtualMachineClass in the web interface:

- Go to the "System" tab, then to the "Virtualization" → "VM Classes" section.
- Click the "Create" button.
- In the window that opens, enter a name for the VM class in the "Name" field.

Next, let's take a closer look at the setting blocks.

### vCPU settings

The `.spec.cpu` block allows you to specify or configure the vCPU for the VM.

{% alert level="warning" %}
Settings in the `.spec.cpu` block cannot be changed after the VirtualMachineClass resource is created.
{% endalert %}

Examples of the `.spec.cpu` block settings:

- A class with a vCPU with the required set of processor instructions. In this case, use `type: Features` to specify the required set of supported instructions for the processor:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

  How to configure vCPU in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Features` in the "Type" field.
  - In the "Required set of supported instructions" field, select the instructions you need for the processor.
  - To create a VM class, click the "Create" button.

- A class with a universal vCPU for a given set of nodes. In this case, use `type: Discovery`:

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

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Discovery` in the "Type" field.
  - Click "Add" in the "Conditions for creating a universal processor" → "Labels and expressions" block.
  - In the pop-up window, you can set the "Key", "Operator" and "Value" of the key that corresponds to the `spec.cpu.discovery.nodeSelector` settings.
  - To confirm the key parameters, click the "Enter" button.
  - To create a VM class, click the "Create" button.

- The vmclass with `type: Host` uses a virtual vCPU that matches the platform node's vCPU instruction set as closely as possible, ensuring high performance and functionality. It also guarantees compatibility with live migration for nodes with similar vCPU types. For example, it is not possible to migrate a virtual machine between nodes with Intel and AMD processors. This also applies to processors of different generations, as their instruction sets may differ.

  ```yaml
  spec:
    cpu:
      type: Host
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Host` in the "Type" field.
  - To create a VM class, click the "Create" button.

- A vmclass with `type: HostPassthrough` uses a physical CPU of the platform node without modification. A virtual machine using this class can only be migrated to a node that has a CPU that exactly matches the CPU of the source node.

  ```yaml
  spec:
    cpu:
      type: HostPassthrough
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `HostPassthrough` in the "Type" field.
  - To create a VM class, click the "Create" button.

- To create a vCPU of a specific CPU with a predefined instruction set, use `type: Model`. To get a list of supported CPU names for the cluster node, run the command in advance:

  ```bash
  d8 k get nodes <node-name> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model.node.virtualization.deckhouse.io")) | .key | split("/")[1]' -r
  ```

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

  After that specify the following in the VirtualMachineClass resource specification:

  ```yaml
  spec:
    cpu:
      model: IvyBridge
      type: Model
  ```

  How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

  - In the "CPU Settings" block, select `Model` in the "Type" field.
  - Select the required processor model in the "Model" field.
  - To create a VM class, click the "Create" button.

### Placement settings

The `.spec.nodeSelector` block is optional. It allows you to specify the nodes that will host VMs using this vmclass:

```yaml
  spec:
    nodeSelector:
      matchExpressions:
        - key: node.deckhouse.io/group
          operator: In
          values:
          - green
```

{% alert level="warning" %}
Since changing the `.spec.nodeSelector` parameter affects all virtual machines using this `VirtualMachineClass`, consider the following:

- For the Enterprise edition, this may cause virtual machines to be migrated to new destination nodes if the current nodes do not meet placement requirements.
- For the Community edition, this may cause virtual machines to restart according to the automatic change application policy set in the `.spec.disruptions.restartApprovalMode` parameter.
{% endalert %}

How to perform the operation in the web interface in the [VM class creation form](#virtualmachineclass-settings):

- Click "Add" in the "VM scheduling conditions on nodes" → "Labels and expressions" block.
- In the pop-up window, you can set the "Key", "Operator" and "Value" of the key that corresponds to the `spec.nodeSelector` settings.
- To confirm the key parameters, click the "Enter" button.
- To create a VM class, click the "Create" button.

### Sizing policy settings

The `.spec.sizingPolicy` block allows you to set sizing policies for virtual machine resources that use vmclass.

{% alert level="warning" %}
Changes in the `.spec.sizingPolicy` block can also affect virtual machines. For virtual machines whose sizing policy will not meet the new policy requirements, the `SizingPolicyMatched` condition in the `.status.conditions` block will be false (`status: False`).

When configuring `sizingPolicy`, be sure to consider the [CPU topology](../../../user/resource-management/virtual-machines.html#automatic-cpu-topology-configuration) for virtual machines.
{% endalert %}

The `cores` block is mandatory and specifies the range of cores to which the rule described in the same block applies.

The ranges [min; max] for the cores parameter must be strictly sequential and non-overlapping.

Correct structure (the ranges follow one another without intersections):

```yaml
- cores:
    min: 1
    max: 4...

- cores:
    min: 5   # Start of next range = (previous max + 1)
    max: 8
```

Invalid option (overlapping values):

```yaml
- cores:
    min: 1
    max: 4...

- cores:
    min: 4   # Error: Value 4 is already included in the previous range
    max: 8
```

{% alert level="warning" %}
Rule: Each new range must start with a value that immediately follows the max of the previous range.
{% endalert %}

Additional requirements can be specified for each range of cores:

1. Memory — specify:

    - Either minimum and maximum memory for all cores in the range,
    - Either the minimum and maximum memory per core (`memoryPerCore`).

2. Allowed fractions of cores (`coreFractions`) — a list of allowed values (for example, [25, 50, 100] for 25%, 50%, or 100% core usage).

{% alert level="warning" %}
For each range of cores, be sure to specify:

- Either `memory` (or `memoryPerCore`).
- Either `coreFractions`.
- Or both parameters at the same time.
{% endalert %}

Here is an example of a policy with similar settings:

```yaml
spec:
  sizingPolicies:
    # For a range of 1–4 cores, it is possible to use 1–8 GB of RAM in 512Mi increments,
    # i.e., 1 GB, 1.5 GB, 2 GB, 2.5 GB, etc.
    # No dedicated cores are allowed.
    # All `corefraction` options are available.
    - cores:
        min: 1
        max: 4
      memory:
        min: 1Gi
        max: 8Gi
        step: 512Mi
      dedicatedCores: [false]
      coreFractions: [5, 10, 20, 50, 100]
    # For a range of 5–8 cores, it is possible to use 5–16 GB of RAM in 1 GB increments,
    # i.e., 5 GB, 6 GB, 7 GB, etc.
    # No dedicated cores are allowed.
    # Some `corefraction` options are available.
    - cores:
        min: 5
        max: 8
      memory:
        min: 5Gi
        max: 16Gi
        step: 1Gi
      dedicatedCores: [false]
      coreFractions: [20, 50, 100]
    # For a range of 9–16 cores, it is possible to use 9–32 GB of RAM in 1 GB increments.
    # You can use dedicated cores if needed.
    # Some `corefraction` options are available.
    - cores:
        min: 9
        max: 16
      memory:
        min: 9Gi
        max: 32Gi
        step: 1Gi
      dedicatedCores: [true, false]
      coreFractions: [50, 100]
    # For the range of 17–248 cores, it is possible to use 1–2 GB of RAM per core.
    # Only the dedicated cores are available for use.
    # The only available `corefraction` parameter is 100%.
    - cores:
        min: 17
        max: 248
      memory:
        perCore:
          min: 1Gi
          max: 2Gi
      dedicatedCores: [true]
      coreFractions: [100]
```

How to configure sizing policies in the web interface in the [VM class creation form](#virtualmachineclass-settings):

- Click "Add" in the "Resource allocation rules for virtual machines" block.
- In the "CPU" block, enter `1` in the "Min" field.
- In the "CPU" block, enter `4` in the "Max" field.
- In the "CPU" block, select the values `5%`, `10%`, `20%`, `50%`, `100%` in order in the "Allow setting core fractions" field.
- In the "Memory" block, set the switch to "Amount per core".
- In the "Memory" block, enter `1` in the "Min" field.
- In the "Memory" block, enter `8` in the "Max" field.
- In the "Memory" block, enter `1` in the "Sampling step" field.
- You can add more ranges using the "Add" button.
- To create a VM class, click the "Create" button.

## vCPU Discovery configuration example

![VirtualMachineClass configuration example](/../../../../../images/virtualization-platform/vmclass-examples.png)

Let's imagine that we have a cluster of four nodes. Two of these nodes labeled `group=blue` have a "CPU X" processor with three instruction sets, and the other two nodes labeled `group=green` have a newer "CPU Y" processor with four instruction sets.

To optimally utilize the resources of this cluster, it is recommended that you create three additional virtual machine classes (VirtualMachineClass):

- `universal`: This class will allow virtual machines to run on all nodes in the platform and migrate between them. It will use the instruction set for the lowest CPU model to ensure the greatest compatibility.
- `cpuX`: This class will be for virtual machines that should only run on nodes with a "CPU X" processor. VMs will be able to migrate between these nodes using the available "CPU X" instruction sets.
- `cpuY`: This class is for VMs that should only run on nodes with a "CPU Y" processor. VMs will be able to migrate between these nodes using the available "CPU Y" instruction sets.

{% alert level="info" %}
A CPU instruction set is a list of all the instructions that a processor can execute, such as addition, subtraction, or memory operations. They determine what operations are possible, affect program compatibility and performance, and can vary from one generation of processors to the next.
{% endalert %}

Resource configuration examples for a given cluster:

```yaml
---
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: universal
spec:
  cpu:
    discovery: {}
    type: Discovery
  sizingPolicies: { ... }
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
  nodeSelector:
    matchExpressions:
      - key: group
        operator: In
        values: ["blue"]
  sizingPolicies: { ... }
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
  sizingPolicies: { ... }
```
