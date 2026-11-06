---
title: Thick volume
description: "An LVM volume in a thick pool, that is, a pool of the LVM type: the space for such a volume is allocated in full at creation. A thick pool gives performance comparable to the storage device but does not allow snapshots."
lang: en
---

Thick volumes are created in thick pools of [local](glossary.html#local-storage) and [replicated](glossary.html#replicated-storage) storage: the LVM mode of the [`sds-local-volume`](/modules/sds-local-volume/) module and LVM volume groups without a thin pool definition in the [`sds-replicated-volume`](/modules/sds-replicated-volume/) module. For volumes that need snapshots, use a [thin volume](glossary.html#thin-volume).
