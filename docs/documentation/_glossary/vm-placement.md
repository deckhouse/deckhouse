---
title: VM placement rules
description: The rules that control on which node a virtual machine starts, from nodeSelector, node affinity, and affinity and anti-affinity to other machines and pods. Conditions are hard or soft.
lang: en
---

A hard condition is mandatory: if no node satisfies it, the machine stays in the `Pending` phase. A soft condition is taken into account by the scheduler where possible. All rules, including the `nodeSelector` of the VM class, apply together. Nodes with taints need matching tolerations on the machine.

When the rules of a running machine change and its current node no longer meets them, commercial editions move the machine by [live migration](glossary.html#live-migration), and in Deckhouse Platform Open the change applies after a reboot. The placement rules of the machine, its [disks](glossary.html#vm-disk) and its class are also checked when a [block device](glossary.html#vm-block-device) is attached. [VM rebalancing](glossary.html#vm-rebalancing) restores the correct placement when the node stops meeting the rules.

For details, see [Placing virtual machines on nodes](user/virtualization/vm-placement.html).
