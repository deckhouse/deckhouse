---
title: "Cloud provider — Azure: Layouts"
description: "Schemes of placement and interaction of resources in Azure when working with the Deckhouse cloud provider."
---

> **Caution!** Only [regions](https://docs.microsoft.com/en-us/azure/availability-zones/az-region) where `Availability Zones` are available are supported.

## Standard

* A separate [resource group](https://docs.microsoft.com/en-us/azure/azure-resource-manager/management/manage-resource-groups-portal) is created for the cluster.
* By default, one external IP address is dynamically allocated to each instance (it is used for Internet access only). Each IP has 64000 ports available for SNAT.
* The [NAT Gateway](https://docs.microsoft.com/en-us/azure/virtual-network/nat-overview) ([pricing](https://azure.microsoft.com/en-us/pricing/details/virtual-network/)) is supported. With it, you can use static public IP addresses for SNAT.
* Public IP addresses can be assigned to master nodes and nodes created by Terraform.
* If the master does not have a public IP, then an additional instance with a public IP (aka bastion host) is required for installation tasks and access to the cluster. In this case, you will also need to configure peering between the cluster's VNet and bastion's VNet.
* Peering can also be configured between the cluster VNet and other VNets.

Example of the layout configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>" # Required.
vNetCIDR: 10.50.0.0/16 # Required.
subnetCIDR: 10.50.0.0/24 # Required.
standard:
  natGatewayPublicIpCount: 1 # Optional, by default 0.
masterNodeGroup:
  replicas: 1
  zones: ["1"] # Optional, by default ["1"].
  instanceClass:
    machineSize: Standard_F4 # Required.
    diskSizeGb: 32
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140 # Required.
    enableExternalIP: false # Optional, by default true.
provider:
  subscriptionId: "<SUBSCRIPTION_ID>" # Required.
  clientId: "<CLIENT_ID>" # Required.
  clientSecret: "<CLIENT_SECRET>" # Required.
  tenantId: "<TENANT_ID>" # Required.
  location: "westeurope" # Required.
# Optional, list of Azure VNets with which Kubernetes VNet will be peered.
peeredVNets:
  - resourceGroupName: kube-bastion # Required.
    vnetName: kube-bastion-vnet # Required.
```
