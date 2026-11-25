---
title: Network configuration in Huawei Cloud
permalink: en/admin/integrations/private/huaweicloud/network.html
lang: en
---

## Security groups

The `internalNetworkSecurity` parameter for the [Standard](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-standard-internalnetworksecurity) and [VpcPeering](/modules/cloud-provider-huaweicloud/cluster_configuration.html#huaweicloudclusterconfiguration-vpcpeering-internalnetworksecurity) layouts (default `true`) enables the creation of a security group (SecurityGroup) when a cluster is created. Deckhouse Platform creates the group named after the cluster prefix and assigns it to the nodes.

The following inbound rules will be created:

- Allow incoming traffic over the TCP protocol on port `22` from `0.0.0.0/0`.
- Allow incoming traffic over the ICMP protocol from `0.0.0.0/0`.
- Allow incoming traffic over the TCP protocol on ports `30000`–`32767` for services of the `NodePort` type. Inbound UDP traffic to `NodePort` ports is not allowed by default.

Attach custom security groups for [CloudEphemeral nodes](../../../../glossary.html#cloud-ephemeral-node) in the [HuaweiCloudInstanceClass](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass) resource via [`spec.securityGroups`](/modules/cloud-provider-huaweicloud/cr.html#huaweicloudinstanceclass-v1-spec-securitygroups). They are applied together with the group created by Deckhouse Platform.
