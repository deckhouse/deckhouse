---
title: "Cloud provider — Basis Dynamix: FAQ"
---

## How to configure a LoadBalancer?

To configure a Service of the LoadBalancer type, add the following annotations to the Service manifest:

```yaml
metadata:
  annotations:
    dynamix.cpi.flant.com/internal-network-name: <internal_name>
    dynamix.cpi.flant.com/external-network-name: <external_name>
```

Both annotations are required:

- `dynamix.cpi.flant.com/internal-network-name` — the name of the internal network in Basis Dynamix
- `dynamix.cpi.flant.com/external-network-name` — the name of the external network in Basis Dynamix

The terms "internal network" and "external network" are used in the context of Basis Dynamix. The external network does not have to be public and may use private IP addresses.

If one of the annotations is not specified, cloud-controller-manager will fail to process the Service.

## How is the placement of a disk chosen?

By the name of a storage policy, and by nothing else. The name is set in two places:

- [`storagePolicy`](cluster_configuration.html#dynamixclusterconfiguration-storagepolicy) at the root of DynamixClusterConfiguration — the policy of every virtual machine in the cluster;
- `storagePolicy` in an instanceClass — in [`masterNodeGroup.instanceClass`](cluster_configuration.html#dynamixclusterconfiguration-masternodegroup-instanceclass-storagepolicy), in [`nodeGroups[].instanceClass`](cluster_configuration.html#dynamixclusterconfiguration-nodegroups-instanceclass-storagepolicy), or in a [DynamixInstanceClass](cr.html#dynamixinstanceclass-v1-spec-storagepolicy) — overrides the cluster-wide value for that node group only.

A storage policy describes a set of storage endpoint and pool pairs available to the account, together with an IOPS limit. Which of those pairs a particular disk ends up on is decided by Basis Dynamix, not by Deckhouse: the module names the policy and leaves the placement to the platform. There is no parameter to pick a storage endpoint or a pool, and the module never moves an existing disk to another placement.

That is also why changing the storage policy — cluster-wide or in a single instanceClass — recreates the CloudEphemeral nodes it applies to.

## Which storage classes does the module create?

One storage class per storage policy that is available to the cluster account and has the `ENABLED` status, named after the policy. A policy name that isn't a valid Kubernetes object name is converted into one, so two policies can end up claiming the same storage class name; in that case only one of them gets a class.

To keep some of them out of the cluster, list the names or regular expressions in the [`storageClass.exclude`](configuration.html#parameters-storageclass-exclude) parameter.
