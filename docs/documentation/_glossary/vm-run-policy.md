---
title: VM run policy
description: The policy that determines how Deckhouse Platform maintains the state of a virtual machine, set by the runPolicy parameter of the machine.
lang: en
---

The allowed values: `AlwaysOnUnlessStoppedManually` (the default, the machine always runs and can be stopped only manually), `AlwaysOn` (the machine is started again even after a shutdown from the guest OS), `Manual` (the user manages the state) and `AlwaysOff` (the machine is always off and cannot be started).

The state of a [virtual machine](glossary.html#virtual-machine) is managed by [VirtualMachineOperation](/modules/virtualization/cr.html#virtualmachineoperation) or by the `d8 v start`, `d8 v stop`, `d8 v restart` and `d8 v evict` commands, which create the same operation.

For details, see [Connecting to a virtual machine](user/virtualization/vm-access.html).
