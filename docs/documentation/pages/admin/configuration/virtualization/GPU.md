---
title: "GPU devices in virtual machines"
permalink: en/admin/configuration/virtualization/gpu-devices.html
description: "GPU device passthrough to virtual machines: cluster requirements, the gpu module, and the GPUClass resource."
search: GPU devices, GPU passthrough, GPUClass, graphics adapter
---

{% alert level="warning" %}
GPU device passthrough is an experimental feature available in commercial DP editions.
{% endalert %}

DP attaches physical GPU devices to virtual machines using DRA (Dynamic Resource Allocation). A project owner requests a device by a reference to a `GPUClass` in the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block of their machine, and you prepare the cluster for this.

To make passthrough work, provide the following:

- [Kubernetes](/products/kubernetes-platform/documentation/v1/reference/supported_versions.html#kubernetes) 1.34 or later with the DRA feature gates required by your cluster configuration.
- The `GPU` feature gate in the module settings.
- A GPU DRA provider installed in the cluster that publishes devices with the `gpu.deckhouse.io` attributes.
- A `GPUClass` resource that selects devices of the model you need. The `gpu` module creates a DeviceClass resource with the same name from it, and the device is allocated to a machine through that class.

To enable the feature gate, add it to the module settings:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: virtualization
spec:
  settings:
    featureGates:
      - GPU
```

After that, tell the project owners the names of the available `GPUClass` resources. A single machine takes no more than 16 devices, and a change to the [`.spec.gpus`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-gpus) block applies only after the machine restarts.
