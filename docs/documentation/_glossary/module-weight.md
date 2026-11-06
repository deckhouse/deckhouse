---
title: Module weight
description: A number from the weight parameter of module.yaml that defines the module startup order. Modules with a lower weight start earlier, the default is 900.
lang: en
---

If `module.yaml` is missing, the weight is taken from the numeric prefix of the module directory, for example `040-node-manager` means 40. If it cannot be obtained, 100 is used. The [Deckhouse controller](glossary.html#deckhouse-controller) considers the weight only for [critical modules](glossary.html#critical-module), whose tasks go to the `main` queue in ascending order of weight. The startup order is also affected by [module dependencies](glossary.html#module-dependencies).

For details, see [Queuing mechanism](architecture/deckhouse/queues.html).
