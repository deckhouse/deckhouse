---
title: CPU oversubscription
description: Giving the virtual machines on a node more virtual cores than the node physically has. The degree of oversubscription is controlled by the core fraction guaranteed to a virtual machine.
lang: en
---

The `coreFraction` parameter of a virtual machine defines the share of a core's capacity guaranteed to it. The allowed values are set in the [sizing policy](glossary.html#sizing-policy) of the class. With `coreFraction: 20%`, a machine always gets a fifth of a core and can take a whole core when the node has spare resources. When placing a machine, Deckhouse Platform sums the guaranteed shares of all machines on the node, and if the sum exceeds the number of physical cores, the machine does not start on that node.

For details, see [CPU oversubscription for virtual machines](admin/configuration/virtualization/cpu-oversubscription.html).
