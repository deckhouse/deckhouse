---
title: "The linstor module"
description: Deckhouse uses the linstor module to manage a replicated block storage in the Kubernetes cluster.
---

{% alert level="danger" %}
The current version of the module is outdated and is no longer supported. Switch to using the [sds-drbd]() module.
{% endalert %}

{% alert level="warning" %}
The module is guaranteed to work only in the following cases:
- when using the stock kernels that come with [supported distributions](../../supported_versions.html#linux);
- when using a 10 Gbps network.

In all other cases, the module may work, but its full functionality is not guaranteed.
{% endalert %}

This module manages a replicated block storage solution in the cluster using the [LINSTOR](https://linbit.com/linstor/) and the [DRBD](https://linbit.com/drbd/) kernel module.

LINSTOR is an orchestrator, acting as an abstraction layer that:
- automates the creation of volumes using well-known and proven technologies such as LVM and ZFS;
- configures the replication of the volumes using DRBD.

The linstor module makes it easy to use LINSTOR-based storage in your cluster. After enabling the linstor module in the Deckhouse configuration, your cluster will be automatically configured to use LINSTOR. All that remains is to [create storage pools](usage.html#linstor-storage-configuration).

Two modes are supported: **LVM** and **LVMThin**.

Each of them has its advantages and disadvantages, read [FAQ](faq.html#performance-and-reliability-notes-comparison-to-ceph) for more details and comparison.
