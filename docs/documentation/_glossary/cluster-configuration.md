---
title: Cluster configuration (ClusterConfiguration)
description: The structure that holds the general parameters of the cluster, such as the Kubernetes version, network settings, CRI parameters and the cluster type.
lang: en
---

The cluster configuration is defined at installation and edited in a running cluster with the `d8 system edit cluster-configuration` command. After the change is saved, Deckhouse Platform reconciles the cluster state with the new configuration. Some of its parameters are [protected](glossary.html#protected-parameters) and cannot be changed in a running cluster by default.

The `clusterType` parameter defines whether the cluster is [static](glossary.html#static-cluster) or [cloud](glossary.html#cloud-cluster).

For details, see [Deckhouse Platform configuration](admin/configuration/).
