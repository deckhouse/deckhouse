---
title: Virtual machine lifecycle
description: The phases a virtual machine goes through from creation to deletion, shown in its status. Conditions explain why the machine is in its current phase.
lang: en
---

In the `Pending` phase the machine waits for its dependent resources to become ready: [disks](glossary.html#vm-disk), [images](glossary.html#vm-image), the VM class and the secret with the [provisioning](glossary.html#vm-provisioning) script. In the `Starting` phase Deckhouse Platform starts the machine on one of the nodes, in `Running` it works, in `Migrating` it moves to another node by [live migration](glossary.html#live-migration), and the `Terminating` phase is irreversible.

The `.status.conditions` block answers why the machine is in its phase: the conditions ending in `Ready` show what blocks the startup, `Running` reports the reason from the scheduler, `Migratable` shows whether the machine can be moved, `EvictionRequired` warns about [eviction from a node](glossary.html#vm-eviction). The [run policy](glossary.html#vm-run-policy) defines how the machine is kept running.

For details, see [Creating a virtual machine](user/virtualization/vm-create.html).
