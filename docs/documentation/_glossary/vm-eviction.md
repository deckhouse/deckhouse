---
title: VM eviction
description: Moving or restarting a virtual machine to free its node. It is triggered when the node is put into maintenance mode, and a machine that can be moved by live migration is not restarted.
lang: en
---

The `EvictionRequired` condition appears when the node with the machine is put into [node maintenance mode](glossary.html#maintenance-mode). A node that was only cordoned does not trigger it. The condition message says what will happen: a [live migration](glossary.html#live-migration) without stopping the guest OS, a restart by Deckhouse Platform with the administrator's permission, or waiting until the machine is restarted. Until the eviction starts, the condition is only a warning, because maintenance can be cancelled.

After a restart the machine starts on another suitable node, and without one it stays in the `Pending` phase. A user can also evict a machine to another node with `d8 v evict` or a [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) of the `Evict` type.

For details, see [Creating a virtual machine](user/virtualization/vm-create.html).
