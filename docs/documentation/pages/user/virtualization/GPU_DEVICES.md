---
title: "GPU devices in a virtual machine"
permalink: en/user/virtualization/gpu-devices.html
description: "Attaching a GPU device provided by an administrator to a project virtual machine."
search: GPU in a VM, GPU passthrough, GPUClass, graphics adapter
---

{% alert level="warning" %}
GPU device passthrough is an experimental feature available in commercial DP editions.
{% endalert %}

The virtualization module attaches physical GPU devices to virtual machines using DRA (Dynamic Resource Allocation). A device is requested by a reference to a `GPUClass` in the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block of the [VirtualMachine](/modules/virtualization/cr.html#virtualmachine) resource.

An administrator prepares the `GPUClass` resources, so ask them which classes are available in the cluster.

To request a GPU device, add the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block to the machine specification:

```yaml
apiVersion: virtualization.deckhouse.io/v1alpha2
kind: VirtualMachine
metadata:
  name: linux-vm
spec:
  # ... other VM settings ...
  gpus:
    - gpuClassName: nvidia-h100
```

In the `gpuClassName` parameter, specify the name of an existing `GPUClass` resource. To attach several devices, add more elements to the list, their order doesn't matter. A single machine takes no more than 16 devices.

A change to the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block applies only after the virtual machine restarts.
