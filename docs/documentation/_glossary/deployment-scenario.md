---
title: Deployment scenario
description: One of the reference cluster configurations used to estimate the resources needed for the expected load, Minimum, Typical or Increased load.
lang: en
---

The Minimum scenario has one [master node](glossary.html#master-node) and at least one [worker node](glossary.html#worker-node). It suits small projects with low reliability requirements, because the failure of the master node affects the whole cluster. The Typical scenario adds two more master nodes, two [frontend nodes](glossary.html#frontend-node) and two [system nodes](glossary.html#system-node) and tolerates the failure of one master node. The Increased load scenario adds two [monitoring nodes](glossary.html#monitoring-node) for observability under high load.

For details, see [Hardware requirements](guides/hardware-requirements.html).
