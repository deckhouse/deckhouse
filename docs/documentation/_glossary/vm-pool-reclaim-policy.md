---
title: VM pool disk reclaim policy
description: The reclaim policy of a disk template of a virtual machine pool. It sets what happens to a replica disk when the replica is removed from the pool.
lang: en
---

The policy is set in the `virtualDiskTemplates` block of a [virtual machine pool](glossary.html#vm-pool) together with the size and the data source of the replica [disk](glossary.html#vm-disk). With `Delete`, the default, the disk is deleted together with the replica. With `Retain`, the disk is kept and reused by the next replica.

For details, see [Virtual machine pools](user/virtualization/vm-pools.html).
