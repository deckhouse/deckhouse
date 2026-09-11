---
title: "Virtual machine networking"
permalink: en/admin/configuration/network/vm-network.html
description: "Subnets that virtual machines get IP addresses from, and the rules for changing them in the virtualization module settings."
search: virtual machine networking, virtualMachineCIDRs, VM subnets, IP addresses
---

The [`.spec.settings.virtualMachineCIDRs`](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) block lists the subnets in CIDR notation from which DP assigns IP addresses to virtual machines, either automatically or on request.
Specify the subnet start address aligned to the mask, for example `192.168.1.192/27`, not an arbitrary address from the range.

Example:

```yaml
spec:
  settings:
    virtualMachineCIDRs:
      - 10.66.10.0/24
      - 10.66.20.0/24
      - 10.77.20.0/16
```

The first and the last address of each subnet are reserved and never assigned to virtual machines. For example, in the `10.66.10.0/24` subnet, the `10.66.10.0` and `10.66.10.255` addresses are unavailable.

You can leave the block unset. The module still starts, but you can no longer work with virtual machine addresses:

- You can't create or use the [VirtualMachineIPAddress](/modules/virtualization/cr.html#virtualmachineipaddress) resource.
- A virtual machine can't request the `Main` network in the [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) parameter.
- The [`.spec.networks`](/modules/virtualization/cr.html#virtualmachine-v1alpha2-spec-networks) parameter of a virtual machine can't be empty.

{% alert level="warning" %}
The subnets in the [`.spec.settings.virtualMachineCIDRs`](/modules/virtualization/configuration.html#parameters-virtualmachinecidrs) block must not overlap with the cluster node subnets, the service subnet, or the pod subnet (`podCIDR`).

You can't delete a subnet if addresses from it are already assigned to virtual machines. You also can't clear the block once it's set.
{% endalert %}
