---
title: Pod redistribution
description: Periodic eviction of pods that match the active scheduling strategies, so that they are scheduled again according to the current state of the cluster. Controlled by the descheduler module.
lang: en
---

Deckhouse Platform analyzes the state of the cluster every 15 minutes and evicts the matching pods. The evicted pods go through the standard scheduling process again. The [priority class](glossary.html#priority-class) of a pod defines the order of eviction, and the `priorityClassThreshold` parameter of a Descheduler limits eviction to pods with a priority below the threshold. [VM rebalancing](glossary.html#vm-rebalancing) is built on the same mechanism.

For details, see [Scheduler](admin/configuration/app-scaling/pod-eviction/scheduler.html).
