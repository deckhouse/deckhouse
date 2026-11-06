---
title: Priority class
description: One of the PriorityClass objects that Deckhouse Platform creates in the cluster and assigns to its components. When resources are insufficient, the scheduler preempts pods with a lower priority first.
lang: en
---

Applications use a priority class through the `priorityClassName` field. A pod without it is treated as having the lowest priority. The classes range from `system-node-critical` and `system-cluster-critical`, reserved for critical cluster components, through `production-high`, `production-medium`, `production-low` and `staging` to `develop`, the default. Additional classes can be created with a name and a numeric priority value. Priority classes also define the order of eviction during [pod redistribution](glossary.html#pod-redistribution).

For details, see [Priority classes](admin/configuration/app-scaling/pod-eviction/priority-classes.html).
