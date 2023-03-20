---
title: "The virtualization module: configuration"
---

{% include module-bundle.liquid %}

> **Note!** Module depends on the [cni-cilium](../021-cni-cilium/) module. Make sure your cluster is deployed with Cilium as the main CNI plugin.
>
> If cilium works in tunnel mode, enabling this module will result in short downtime due to reconfiguration of overlay network to a non-standard VXLAN port.
>
> **Note!** Module requires kernel version >= `5.7`.

You will also need to specify one or more desired subnets from which IP addresses will be allocated to virtual machines:

```yaml
vmCIDRs:
- 10.10.10.0/24
```

The subnet for the VMs should not conflict with the subnet for the pods and the subnet for the services

{% include module-settings.liquid %}
