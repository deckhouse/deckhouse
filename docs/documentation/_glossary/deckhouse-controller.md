---
title: Deckhouse controller
description: The controller of the deckhouse module that implements platform management operations and coordinates them through the queueing mechanism. The deckhouse module is the only module without which Deckhouse Platform cannot run.
lang: en
---

The Deckhouse controller manages [modules](glossary.html#module): it runs their [hooks](glossary.html#module-hook), applies module settings and installs module releases. Tasks are processed in the [addon-operator queues](glossary.html#addon-operator-queue). In [HA mode](glossary.html#ha-mode) several controller instances run and elect a leader through the `deckhouse-leader-election` Lease. The leader performs all management operations.

The controller can run in standard mode or in hook isolation mode, in which shell hooks and module enable scripts run in a chroot environment with a limited set of mounted directories.

For details, see [Deckhouse module](architecture/deckhouse/deckhouse.html).
