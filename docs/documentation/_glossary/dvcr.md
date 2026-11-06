---
title: DVCR
description: The internal container image storage in which Deckhouse Platform keeps virtual machine images. It resides on a persistent volume of the cluster.
lang: en
---

Images travel from DVCR to virtual machine disks, so the size of the volume determines how many images fit into the cluster. The volume size and storage class are set in the `dvcr.storage` block of the [`virtualization`](/modules/virtualization/) [ModuleConfig](reference/api/cr.html#moduleconfig). After the volume is created, its size cannot be reduced and its storage class cannot be changed. Stale data of deleted images and disks is removed by garbage collection on a schedule, daily at 02:00 by default.

For details, see [Virtual machine image storage](admin/configuration/virtualization/image-storage.html).
