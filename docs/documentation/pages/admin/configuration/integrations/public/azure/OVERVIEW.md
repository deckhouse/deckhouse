---
title: Integration with Microsoft Azure
permalink: en/admin/integrations/public/azure/overview.html
---

Deckhouse Platform integrates with [Microsoft Azure](https://portal.azure.com/) and provisions cluster nodes in it. Virtual machine parameters are set in the [AzureInstanceClass](/modules/cloud-provider-azure/cr.html#azureinstanceclass) resource, which you reference in a [NodeGroup](/modules/node-manager/cr.html#nodegroup).

Integration features:

- Managing Azure resources:
  - creates network routes for the `PodNetwork` network on the Azure side;
  - creates load balancers for Services of the LoadBalancer type;
  - updates cluster node metadata and removes from cluster nodes that no longer exist in Azure.
- Provisioning disks via the Azure Disk CSI driver (`disk.csi.azure.com`) and creating StorageClasses for Azure disk types so that PersistentVolumes can be requested from the cluster.
- Provisioning base infrastructure and CloudPermanent nodes using the [Terraform/OpenTofu provider](../../../../architecture/cluster-and-infrastructure/cloud-providers/cloud-provider-azure.html#module-interactions) `terraform-provider-azurerm`.
- Provisioning CloudEphemeral nodes via Machine Controller Manager (MCM).
- Enabling CNI in new clusters automatically. By default, [`cni-cilium`](/modules/cni-cilium/) is used.
