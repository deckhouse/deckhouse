---
title: "Virtual Machine Classes"
permalink: en/virtualization-platform/documentation/admin/platform-management/virtualization/virtual-machine-classes.html
---

The [`VirtualMachineClass`](../../../reference/cr/virtualmachineclass.html) resource is intended for centralized configuration of preferred virtual machine parameters. It allows setting parameters for CPU, including instructions and resource configuration policies, as well as defining the ratio between CPU and memory resources. Additionally, [VirtualMachineClass](../../../reference/cr/virtualmachineclass.html) manages the placement of virtual machines across the platform nodes, helping administrators efficiently distribute resources and optimally place virtual machines.

The virtualization platform provides 3 preconfigured `VirtualMachineClass` resources:

```shell
kubectl get virtualmachineclass
NAME               PHASE   AGE
host               Ready   6d1h
host-passthrough   Ready   6d1h
generic            Ready   6d1h
```

- `host` — this class uses a virtual CPU that closely matches the instruction set of the platform node's CPU, ensuring high performance and functionality. It also guarantees compatibility with live migration for nodes with similar processor types. For example, migrating a virtual machine between nodes with Intel and AMD processors is not possible. This also applies to processors from different generations, as their instruction sets may differ.
- `host-passthrough` — in this class, the physical CPU of the platform node is used without modification. A virtual machine using this class can only be migrated to a node with a CPU that exactly matches the CPU of the original node.
- `generic` — a universal CPU class using the Nehalem model, which is quite old but supported by most modern processors. This allows virtual machines to run on any node in the cluster with live migration capabilities.

The [`VirtualMachineClass`](../../../reference/cr/virtualmachineclass.html) is a mandatory parameter in the virtual machine configuration. Here's an example of how to specify the virtual machine class in the specification:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  virtualMachineClassName: generic # name of resource VirtualMachineClass
  ...
```

{% alert level="warning" %}
It is recommended to create at least one [`VirtualMachineClass`](../../../../reference/cr/virtualmachineclass.html) resource in the cluster with the Discovery type as soon as all nodes are configured and added to the cluster. This will allow virtual machines to use a universal processor with the maximum possible characteristics considering the CPUs on the cluster nodes, ensuring that virtual machines can fully utilize CPU capabilities and, if necessary, migrate seamlessly between cluster nodes. Examples and descriptions of classes with the Discovery type are provided below.
{% endalert %}

Platform administrators can create virtual machine classes according to their needs, but it is recommended to minimize the number of them to simplify management. Below is an example of the configuration.

### Example Configuration of VirtualMachineClass

![Example Configuration of VirtualMachineClass](/images/virtualization-platform/vmclass-examples.png)

Suppose we have a cluster of four nodes. Two of these nodes with the label `group=blue` are equipped with **CPU X**, which supports three instruction sets. The other two nodes with the label `group=green` have a newer processor, **CPU Y**, which supports four instruction sets. In this case, the administrator can configure virtual machine classes to ensure compatibility with different types of processors in the cluster.

To optimally use the resources of this cluster, it is recommended to create three additional virtual machine classes (`VirtualMachineClass`):

- **universal**: This class will allow virtual machines to run on all nodes of the platform and migrate between them. The instruction set for the lowest model CPU will be used, ensuring maximum compatibility.
- **cpuX**: This class will be dedicated to virtual machines that should run only on nodes with the "CPU X" processor. VMs will be able to migrate between these nodes, using the available instruction sets of "CPU X".
- **cpuY**: This class is for virtual machines that should run only on nodes with the "CPU Y" processor. VMs will be able to migrate between these nodes, using the available instruction sets of "CPU Y".

> Instruction sets for a processor are a list of all the commands the processor can execute, such as addition, subtraction, or memory operations. They define which operations are possible, affect program compatibility and performance, and can change from one processor generation to another.

Sample resource configurations for this cluster:

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
    discovery: {}
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

### Other configuration options

Example configuration for the `VirtualMachineClass` resource:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: discovery
spec:
  cpu:
    # configure a universal vCPU for a specific set of nodes
    discovery:
      nodeSelector:
        matchExpressions:
          - key: node-role.kubernetes.io/control-plane
            operator: DoesNotExist
    type: Discovery
  # allow virtual machines with this class to run only on nodes in the worker group.
  nodeSelector:
    matchExpressions:
      - key: node.deckhouse.io/group
        operator: In
        values:
          - worker
  # resource configuration policy
  sizingPolicies:
  # for a range of 1 to 4 cores, it is possible to allocate between 1 and 8 GB of RAM in 512Mi increments
  # for example, 1GB, 1.5GB, 2GB, 2.5GB, and so on
  # dedicated cores are not allowed
  # all values of the corefraction parameter are permitted
    - cores:
        min: 1
        max: 4
      memory:
        min: 1Gi
        max: 8Gi
        step: 512Mi
      dedicatedCores: [false]
      coreFractions: [5, 10, 20, 50, 100]
  # for a range of 5 to 8 cores, it is possible to allocate between 5 and 16 GB of RAM in 1GB increments
  # for example, 5GB, 6GB, 7GB, and so on
  # dedicated cores are not allowed
  # some values of the corefraction parameter are permitted
    - cores:
        min: 5
        max: 8
      memory:
        min: 5Gi
        max: 16Gi
        step: 1Gi
      dedicatedCores: [false]
      coreFractions: [20, 50, 100]
  # for a range of 9 to 16 cores, it is possible to allocate between 9 and 32 GB of RAM in 1GB increments
  # dedicated cores can be used (but are not mandatory)
  # some values of the corefraction parameter are permitted
    - cores:
        min: 9
        max: 16
      memory:
        min: 9Gi
        max: 32Gi
        step: 1Gi
      dedicatedCores: [true, false]
      coreFractions: [50, 100]
  # for a range of 17 to 1024 cores, it is possible to allocate between 1 and 2 GB of RAM per core
  # only dedicated cores are allowed
  # the only permitted corefraction value is 100%
    - cores:
        min: 17
        max: 1024
      memory:
        perCore:
          min: 1Gi
          max: 2Gi
      dedicatedCores: [true]
      coreFractions: [100]
```

Here are fragments of `VirtualMachineClass` configurations for solving various tasks:

- A class with vCPU that requires a specific set of processor instructions. To achieve this, we use `type: Features` to specify the necessary set of supported instructions for the processor:

  ```yaml
  spec:
    cpu:
      features:
        - vmx
      type: Features
  ```

- A class with a universal vCPU for a given set of nodes. To achieve this, we use `type: Discovery`:

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

- To create a vCPU of a specific processor with a predefined set of instructions, use `type: Model`. First, to get a list of supported CPU model names for a cluster node, run the following command:

  ```shell
  kubectl get nodes <node-name> -o json | jq '.metadata.labels | to_entries[] | select(.key | test("cpu-model")) | .key | split("/")[1]' -r

  # Example output:
  #
  # IvyBridge
  # Nehalem
  # Opteron_G1
  # Penryn
  # SandyBridge
  # Westmere
  ```

- Then, specify it in the VirtualMachineClass resource specification:

  ```yaml
  spec:
    cpu:
      model: IvyBridge
      type: Model
  ```
  
