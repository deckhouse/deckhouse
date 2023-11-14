---
title: "The linstor module"
description: Deckhouse uses the linstor module to manage a replicated block storage in the Kubernetes cluster.
---

{% alert level="warning" %}
Работоспособность модуля гарантируется только в следующих случаях:
- при использовании стоковых ядер, поставляемых вместе с [поддерживаемыми дистрибутивами](../../supported_versions.html#linux);
- при использовании сети 10Gbps.

Работоспособность модуля в других условиях возможна, но не гарантируется.
{% endalert %}

This module manages a replicated block storage solution in the cluster using the [LINSTOR](https://linbit.com/linstor/) and the [DRBD](https://linbit.com/drbd/) kernel module.

LINSTOR is an orchestrator, acting as an abstraction layer that:
- automates the creation of volumes using well-known and proven technologies such as LVM and ZFS;
- configures the replication of the volumes using DRBD.

The linstor module makes it easy to use LINSTOR-based storage in your cluster. After enabling the linstor module in the Deckhouse configuration, your cluster will be automatically configured to use LINSTOR. All that remains is to [create storage pools](configuration.html#linstor-storage-configuration).

Two modes are supported: **LVM** and **LVMThin**.

Each of them has its advantages and disadvantages, read [FAQ](faq.html#performance-and-reliability-notes-comparison-to-ceph) for more details and comparison.
