---
title: "Placement of virtual machines across nodes"
permalink: en/admin/configuration/virtualization/vm-classes-placement.html
description: "VirtualMachineClass rules that limit the choice of nodes for virtual machines of that class."
search: node placement, nodeSelector, tolerations, virtual machine class
---

The optional [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-nodeselector) block limits the set of nodes where virtual machines of this class run. Nodes are selected by labels:

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
A change to the [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-nodeselector) block affects all virtual machines of the class at once. Those running on nodes that no longer match the new conditions have to be moved:

- In commercial editions, DP migrates such VMs to suitable nodes.
- In DP Open, the VMs are restarted, and the restart time depends on the [`.spec.disruptions.restartApprovalMode`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-disruptions-restartapprovalmode) parameter of the virtual machine, which defaults to `Manual` and requires the project owner's approval.
{% endalert %}

To do the same in the web interface, in the [VM class creation form](vm-classes.html#virtualmachineclass-settings):

1. Click **Add** in the **Conditions for scheduling VMs on nodes** → **Labels and expressions** block.
1. Set **Key**, **Operator**, and **Value**. They correspond to the [`.spec.nodeSelector`](/modules/virtualization/cr.html#virtualmachineclass-v1alpha3-spec-nodeselector) parameter.
1. Press **Enter** to confirm the key parameters.
1. Click **Create**.
