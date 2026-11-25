---
title: Storage and load balancing in Microsoft Azure
permalink: en/admin/integrations/public/azure/load-balancing-and-storage.html
---

## Storage

When running on Azure, Deckhouse Platform automatically creates the following StorageClass resources:

| Name                    | Disk type        |
| ---------------------- | ---------------- |
| `managed-standard`     | Standard\_LRS    |
| `managed-standard-ssd` | StandardSSD\_LRS |
| `managed-premium`      | Premium\_LRS     |

You can additionally:

- Disable unneeded storage classes using the [`exclude`](/modules/cloud-provider-azure/configuration.html#parameters-storageclass-exclude) parameter.
- Define custom StorageClass resources with the target throughput and IOPS settings.

Example configuration in a ModuleConfig:

```yaml
apiVersion: deckhouse.io/v1alpha1
kind: ModuleConfig
metadata:
  name: cloud-provider-azure
spec:
  version: 1
  enabled: true
  settings:
    storageClass:
      provision:
      - name: managed-ultra-ssd
        type: UltraSSD_LRS
        diskIOPSReadWrite: 600
        diskMBpsReadWrite: 150
      exclude:
      - managed-standard.*
      - managed-premium
```

## Load balancing

Deckhouse Platform automatically creates LoadBalancer resources in Azure when using Kubernetes Service objects of the LoadBalancer type.

{% alert level="info" %}
All outgoing traffic from the cluster comes through LoadBalancers.
If no LoadBalancer is configured to handle UDP traffic, all outgoing UDP traffic will be blocked,
which can affect NTP utilities such as `ntpdate`, `chrony`, etc.
To resolve this, manually add a rule for any UDP port to an existing LoadBalancer or create a Kubernetes Service of the LoadBalancer type with a UDP port.
{% endalert %}
