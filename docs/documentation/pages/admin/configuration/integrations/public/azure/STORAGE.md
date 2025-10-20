---
title: Storage and load balancing
permalink: en/admin/integrations/public/azure/storage.html
---

## Storage

When running on Azure, Deckhouse Kubernetes Platform (DKP) automatically creates the following StorageClass resources:

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

DKP automatically creates LoadBalancer resources in Azure when using Kubernetes Service objects of the LoadBalancer type.

Additional features:

- NAT Gateway is supported.
  You can explicitly define the number of public IP addresses for SNAT using the [`natGatewayPublicIpCount`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-standard-natgatewaypublicipcount) parameter.
- VNet peering can be configured, including with a bastion host or other VNets in the cloud.
- Service Endpoints are supported.
  They provide secure and direct connections to Azure services without using public IPs.
  For more details, refer to [Integration with Microsoft Azure services](services.html).
