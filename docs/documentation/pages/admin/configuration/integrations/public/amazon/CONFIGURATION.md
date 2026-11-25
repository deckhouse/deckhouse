---
title: Integration configuration in Amazon Web Services
permalink: en/admin/integrations/public/amazon/configuration.html
lang: en
---

## Defining AWSClusterConfiguration

The [AWSClusterConfiguration](/modules/cloud-provider-aws/cluster_configuration.html#awsclusterconfiguration) resource describes the cluster settings and is used by Deckhouse Platform to:

- Define the layout and network CIDRs.
- Configure master and [worker nodes](../../../../glossary.html#worker-node).
- Specify AWS API connection parameters (such as access keys and region).
- Define common and custom tags.
- Configure the [bastion host](../../../../glossary.html#bastion-host) (for the WithNAT [layout](../../../../glossary.html#layout)).

Required fields:

- `apiVersion`: Must be set to `deckhouse.io/v1`.
- `kind`: Always AWSClusterConfiguration.

Example resource header:

```yaml
apiVersion: deckhouse.io/v1
kind: AWSClusterConfiguration
```

To edit this resource in a running cluster, run the following command:

```shell
d8 system edit provider-cluster-configuration
```

After editing, apply the changes with the following command:

```shell
dhctl converge
```
