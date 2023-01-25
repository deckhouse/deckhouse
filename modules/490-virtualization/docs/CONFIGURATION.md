---
title: "The virtualization module: configuration"
---

{% include module-bundle.liquid %}

> **Important:**  Module depends on cni-cilium. Make sure your cluster is deployed with Cilium as the main CNI plugin.

You will also need to specify one or more desired subnets from which IP addresses will be allocated to virtual machines:

```yaml
vmCIDRs:
- 10.10.10.0/24
```

The subnet for the VMs should not conflict with the subnet for the pods and the subnet for the services

{% include module-settings.liquid %}
