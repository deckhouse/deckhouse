---
title: Network configuration in Microsoft Azure
permalink: en/admin/integrations/public/azure/network.html
lang: en
---

## Network configuration

- NAT Gateway is supported.
  You can explicitly define the number of public IP addresses for SNAT using the [`natGatewayPublicIpCount`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-standard-natgatewaypublicipcount) parameter.
- VNet peering can be configured, including with a [bastion host](../../../../glossary.html#bastion-host) or other VNets in the cloud.

## Security groups

When a cluster is created, Deckhouse Platform creates a Network Security Group (NSG) named after the cluster prefix and associates it with the node subnet.

The following rules will be created:

- `AllowIcmp`: Allows incoming traffic over the ICMP protocol from any source.
- `AllowSsh`: Allows incoming traffic over the TCP protocol on port `22` from the CIDRs listed in [`sshAllowList`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-sshallowlist). If the list is not set, traffic is allowed from any source.

You cannot attach a pre-created custom NSG to nodes through the Deckhouse Platform parameters. To restrict SSH access, use [`sshAllowList`](/modules/cloud-provider-azure/cluster_configuration.html#azureclusterconfiguration-sshallowlist).
