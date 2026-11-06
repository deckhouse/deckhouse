---
title: Thin volume
description: "An LVM volume in a thin pool, that is, a pool of the LVMThin type: the space for such a volume is allocated as it fills up. A thin pool allows snapshots but has lower performance."
lang: en
---

Thin volumes are created in thin pools of [local](glossary.html#local-storage) and [replicated](glossary.html#replicated-storage) storage: the LVMThin mode of the [`sds-local-volume`](/modules/sds-local-volume/) module and LVM volume groups with a thin pool definition in the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module. Where snapshots are not needed, a [thick volume](glossary.html#thick-volume) gives higher performance.
