---
title: "The linstor module"
---

This module manages a replicated block storage solution in the cluster using the [LINSTOR](https://linbit.com/linstor/) and the [DRBD](https://linbit.com/drbd/) kernel module.

LINSTOR is neither a file system nor block storage. LINSTOR is an orchestrator, acting as an abstraction layer that: 
- automates the creation of volumes using well-known and proven technologies such as LVM and ZFS; 
- configures the replication of the volumes using DRBD.

The linstor module makes it easy to use LINSTOR-based storage in your cluster. After enabling the linstor module in the Deckhouse configuration, your cluster will be automatically configured to use LINSTOR. All that remains is to [create storage pools](configuration.html#linstor-storage-configuration).

Two modes are supported: **LVM** and **LVMThin**.

Each of them has its advantages and disadvantages, read [FAQ](faq.html#performance-and-reliability-notes-comparison-to-ceph) for more details and comparison. 
