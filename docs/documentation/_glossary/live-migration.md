---
title: Live migration
description: Moving a running virtual machine from one node to another without shutting it down.
lang: en
---

Live migration is used for load balancing, to free a node for [maintenance](glossary.html#maintenance-mode) or update, and for a virtual machine firmware update that would otherwise require a restart. A node sends the memory of one virtual machine at a time and accepts one incoming migration at a time, and the transfer rate of a single migration is limited to 640 MiB/s. [VM rebalancing](glossary.html#vm-rebalancing) also moves machines with live migration.

For details, see [Maintenance of nodes running virtual machines](admin/configuration/platform-scaling/node/vm-node-maintenance.html).
