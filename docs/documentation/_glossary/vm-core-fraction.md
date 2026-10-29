---
title: Core fraction
description: The guaranteed share of the power of each virtual core of a virtual machine, set by the coreFraction parameter. Without it every virtual core gets 100% of a physical one.
lang: en
---

A machine with `cores: 2` and `coreFraction: 20%` is guaranteed 0.4 cores in total regardless of the node load and can take both cores in full when the node has free resources. Such a reserve keeps more machines on a node than it has physical cores. This is how [CPU oversubscription](glossary.html#cpu-oversubscription) works. The guaranteed share is taken into account when a node is selected, so a machine does not start where the node cannot provide the guarantees to all machines placed on it.

An administrator can restrict the allowed values in the [sizing policy](glossary.html#sizing-policy) of the VM class. In commercial editions `coreFraction: Auto` lets Deckhouse Platform pick the share by the actual consumption of the machine.

For details, see [Virtual machine CPU and memory](user/virtualization/vm-resources.html).
