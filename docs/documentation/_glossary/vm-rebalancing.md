---
title: VM rebalancing
description: Restoring an even distribution of virtual machines across nodes by moving them with live migration. Performed by the descheduler module.
lang: en
---

Rebalancing evens out the load: when a node reserves more than 80% of CPU, some machines are moved to nodes that reserve less than 50%. It also restores correct placement when the current node no longer meets the machine requirements or the rules of mutual placement. With the [`descheduler`](/modules/descheduler/) module enabled, Deckhouse Platform creates a Descheduler named [`virtualization`](/modules/virtualization/) and selects in it only the machines that can be moved by [live migration](glossary.html#live-migration).

For details, see [Virtual machine fault tolerance and balancing](admin/configuration/high-reliability-and-availability/vm-reliability.html).
