---
title: Addon-operator queue
description: A task queue of the Deckhouse controller that processes built-in modules and modules from a source. Each queue is a separate pipeline with one worker that executes tasks from the head until they succeed.
lang: en
---

The [Deckhouse controller](glossary.html#deckhouse-controller) creates three kinds of queues:

- the `main` queue, which runs global hooks at startup, installs [critical modules](glossary.html#critical-module) and configures or deletes modules;
- several parallel queues `parallel_queue_0..N` for module runs with [dependencies](glossary.html#module-dependencies) taken into account;
- hook queues named in the configuration of a [hook](glossary.html#module-hook).

A task that fails and has no `allowFailure` blocks its queue, and it is restarted with an exponential delay starting at 5 seconds. Queues do not block each other. The queues can be inspected with `d8 system queue list`.

For details, see [Queuing mechanism](architecture/deckhouse/queues.html).
