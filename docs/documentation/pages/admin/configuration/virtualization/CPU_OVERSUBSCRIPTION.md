---
title: "CPU oversubscription for virtual machines"
permalink: en/admin/configuration/virtualization/cpu-oversubscription.html
description: "CPU oversubscription for virtual machines: a fixed core fraction and a fraction chosen by the project owner through the virtual machine class."
search: CPU oversubscription, coreFraction, CPU overcommit
---

Oversubscription lets you give the virtual machines on a node more virtual cores than the node physically has. This makes sense because VMs rarely load the CPU at the same time and at full capacity.

The degree of oversubscription is controlled by the `coreFraction` parameter of a virtual machine, and you define its allowed values in the sizing policy of the class. The parameter defines the share of a core's capacity guaranteed to a VM. For example, with `coreFraction: 20%`, a VM always gets a fifth of a core, and it can take a whole core when the node has spare resources.

{% alert level="info" %}
If the `coreFractions` list isn't set in the class or contains several values, the project owner chooses the degree of oversubscription by specifying `coreFraction` when creating a VM.
{% endalert %}

When placing a VM on a node, the module sums the guaranteed shares of all VMs on that node using the `Σ(cores × coreFraction / 100)` formula. If the sum exceeds the number of physical cores, the VM doesn't start on that node.

Consider a node with 4 physical cores and 5 VMs, each with 2 cores and `coreFraction: 20%`. The guaranteed load is `5 × 2 × 0.2 = 2` cores, with 10 virtual cores on 4 physical ones, which is an oversubscription of 2.5 to 1. All five VMs fit on the node, because 2 cores is less than the available 4.

## Fixed oversubscription

A list with a single value leaves the project owner no choice, and you define the degree of oversubscription for all VMs of the class:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: oversubscribed
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 8
      memory:
        perCore:
          min: 1Gi
          max: 8Gi
      coreFractions: [20] # The only allowed value.
      defaultCoreFraction: 20
```

All VMs of this class get 20% of a core each, which gives an oversubscription of 5 to 1.

## Oversubscription chosen by the project owner

A list with several values leaves the choice to the project owner:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachineClass
metadata:
  name: standard
spec:
  sizingPolicies:
    - cores:
        min: 1
        max: 4
      memory:
        perCore:
          min: 1Gi
          max: 8Gi
      coreFractions: [5, 10, 20, 50, 100]
      defaultCoreFraction: 20
```

The project owner selects `coreFraction` from the list, and if they don't, the VM gets 20%.
