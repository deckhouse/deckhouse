---
title: Node maintenance mode
description: A node state in which Deckhouse Platform moves the virtual machines to other nodes and does not start new ones on it, so that work on the node does not disrupt them.
lang: en
---

A node is switched to maintenance mode with the `d8 k drain` command or with **Cordon + Drain** in the web interface, and returned to service with `d8 k uncordon`. Virtual machines that can be moved are moved by [live migration](glossary.html#live-migration). A machine that cannot be moved, for example one pinned to the node or using a passed-through device, keeps the node busy until it is restarted. The restart is allowed by the `virtualization.deckhouse.io/virtualmachines-restart-approved` annotation on the node.

For details, see [Maintenance of nodes running virtual machines](admin/configuration/platform-scaling/node/vm-node-maintenance.html).
