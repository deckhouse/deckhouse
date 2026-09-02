---
title: Layouts and configuration in Microsoft Azure
permalink: en/admin/integrations/public/azure/layout.html
---

This section describes the cluster deployment layout in Microsoft Azure infrastructure and the associated parameters.

## Standard

Standard is the supported deployment layout with the following characteristics:

- A separate resource group is created for the cluster.
- Each instance is assigned a public IP address by default (used only for accessing the internet).
- Up to 64,000 SNAT ports are available per public IP address.
- A NAT Gateway is supported and billable.
  It allows using static public IP addresses for SNAT.
- Public IPs can be assigned to both master nodes and nodes created via Terraform.
- If the master node does not have a public IP, a bastion host and VNet peering between clusters are required.
- Peering between the cluster's VNet and other VNets is supported.

The module creates a Network Security Group (NSG) named after the cluster prefix and associates it with the node subnet.

The following rules will be created:

- `AllowIcmp`: Allows incoming traffic over the `ICMP` protocol from any source.
- `AllowSsh`: Allows incoming traffic over the `TCP` protocol on port 22 from the CIDRs listed in [`sshAllowList`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-sshallowlist). If the list is not set, traffic is allowed from any source.

You cannot attach a pre-created custom NSG to nodes through the module parameters. To restrict SSH access, use [`sshAllowList`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-sshallowlist).

Example configuration:

```yaml
apiVersion: deckhouse.io/v1
kind: AzureClusterConfiguration
layout: Standard
sshPublicKey: "<SSH_PUBLIC_KEY>"       # Required.
vNetCIDR: 10.50.0.0/16                 # Required.
subnetCIDR: 10.50.0.0/24               # Required.
standard:
  natGatewayPublicIpCount: 1           # Optional (0 by default).
masterNodeGroup:
  replicas: 1
  zones: ["1"]                         # Optional (["1"] by default).
  instanceClass:
    machineSize: Standard_F4           # Required.
    diskSizeGb: 32
    urn: Canonical:UbuntuServer:18.04-LTS:18.04.202010140  # Required.
    enableExternalIP: false            # Optional (true by default).
provider:
  subscriptionId: "<SUBSCRIPTION_ID>"  # Required.
  clientId: "<CLIENT_ID>"              # Required.
  clientSecret: "<CLIENT_SECRET>"      # Required.
  tenantId: "<TENANT_ID>"              # Required.
  location: "westeurope"               # Required.
peeredVNets:                           # Optional.
  - resourceGroupName: kube-bastion    # Required.
    vnetName: kube-bastion-vnet        # Required.
```
