---
title: Replicated storage
description: Storage in which data is replicated across several nodes, so that it stays available when one of the nodes fails.
lang: en
---

Replicated block StorageClass objects are created with the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module, which is based on Distributed Replicated Block Device (DRBD). Replicated storage is one of the [software-defined storage](glossary.html#sds) types. Its pools are created on LVM volume groups as [thick](glossary.html#thick-volume) or [thin](glossary.html#thin-volume) pools.

For details, see [Setting up replicated storage based on DRBD](admin/configuration/storage/sds/lvm-replicated.html).
