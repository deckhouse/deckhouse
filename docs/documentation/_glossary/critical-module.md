---
title: Critical module
description: A module marked with critical set to true in module.yaml. It starts during the initial cluster bootstrap, before the cluster is considered fully ready. Other modules start only after that.
lang: en
---

The [Deckhouse controller](glossary.html#deckhouse-controller) puts the tasks of critical modules into the `main` [queue](glossary.html#addon-operator-queue) in ascending order of [module weight](glossary.html#module-weight). Non-critical modules are processed in the parallel queues without regard to weight. If critical modules can be processed in parallel, the controller runs them as a batch and waits for completion before continuing the `main` queue.

For details, see [Queuing mechanism](architecture/deckhouse/queues.html).
