---
title: Network configuration in Google Cloud Platform
permalink: en/admin/integrations/public/gcp/network.html
lang: en
---

## Firewall rules

When a cluster is created, Deckhouse Platform creates the following firewall rules in the cluster VPC:

- `<CLUSTER_PREFIX>-ssh-and-ping`: Allows incoming traffic over the ICMP and TCP (port `22`) protocols to nodes with the `<CLUSTER_PREFIX>` network tag from the CIDRs listed in [`sshAllowList`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-sshallowlist) (default `0.0.0.0/0`).
- `<CLUSTER_PREFIX>-intercommunication`: Allows any traffic between nodes with the `<CLUSTER_PREFIX>` network tag, as well as from the pod subnet ([`podSubnetCIDR`](/modules/control-plane-manager/configuration.html#parameters-network-podsubnetcidr)).

Custom firewall rules are applied to nodes through additional network tags ([`additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags)):

- For master nodes, in the [`masterNodeGroup.instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-masternodegroup-instanceclass-additionalnetworktags) parameter of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.
- For [static nodes](../../../../glossary.html#static-node), in the [`nodeGroups[].instanceClass.additionalNetworkTags`](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration-nodegroups-instanceclass-additionalnetworktags) parameter of the [GCPClusterConfiguration](/modules/cloud-provider-gcp/cluster_configuration.html#gcpclusterconfiguration) resource.
- For ephemeral nodes, in the [`spec.additionalNetworkTags`](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass-v1-spec-additionalnetworktags) parameter of the [GCPInstanceClass](/modules/cloud-provider-gcp/cr.html#gcpinstanceclass) resource.
