---
title: ColdStandby
description: A mechanism that returns a virtual machine to service on another node after the failure of the node it was running on.
lang: en
---

The mechanism requires the `runPolicy` of the virtual machine to be `AlwaysOnUnlessStoppedManually` or `AlwaysOn`, and [Fencing](glossary.html#fencing) to be enabled on the nodes that run virtual machines. Without Fencing, an unavailable machine does not move: it stays on the failed node and resumes together with it.

For details, see [Virtual machine fault tolerance and balancing](admin/configuration/high-reliability-and-availability/vm-reliability.html).
