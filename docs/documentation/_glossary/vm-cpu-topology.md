---
title: VM CPU topology
description: The distribution of the processor cores of a virtual machine across sockets. The user sets only the total number of cores, and Deckhouse Platform calculates the number of sockets.
lang: en
---

The more cores, the more sockets they are divided into and the larger the step in which their number can be changed: up to 16 cores use one socket, up to 32 two, up to 64 four and up to 248 eight. The total number of cores has to be a multiple of the number of sockets. The topology matters for applications sensitive to the processor configuration.

The number of cores of a running machine can be changed within the current topology by [live migration](glossary.html#live-migration) when the corresponding feature is enabled. A change that requires a different topology needs a restart.

For details, see [Virtual machine CPU and memory](user/virtualization/vm-resources.html).
